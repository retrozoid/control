package control

import (
	"errors"
	"fmt"

	"github.com/retrozoid/control/protocol/dom"
	"github.com/retrozoid/control/protocol/runtime"
)

var ErrExecutionContextDestroyed = errors.New("execution context destroyed")

type DOMException string

func (e DOMException) Error() string {
	return string(e)
}

type nodeType float64

const (
	nodeTypeElement               nodeType = 1  // An Element node like <p> or <div>
	nodeTypeAttribute             nodeType = 2  // An Attribute of an Element
	nodeTypeText                  nodeType = 3  // The actual Text inside an Element or Attr
	nodeTypeCDataSection          nodeType = 4  // A CDATASection
	nodeTypeProcessingInstruction nodeType = 7  // A ProcessingInstruction of an XML document
	nodeTypeComment               nodeType = 8  // A Comment node
	nodeTypeDocument              nodeType = 9  // A Document node
	nodeTypeDocumentType          nodeType = 10 // A DocumentType node
	nodeTypeFragment              nodeType = 11 // A DocumentFragment node
)

type JsObject interface {
	ObjectID() runtime.RemoteObjectId
}

type RemoteObject runtime.RemoteObjectId

func (r RemoteObject) ObjectID() runtime.RemoteObjectId {
	return runtime.RemoteObjectId(r)
}

func getNodeType(deepSerializedValue any) nodeType {
	return nodeType(deepSerializedValue.(map[string]any)["nodeType"].(float64))
}

func deepUnserialize(self string, value any) any {
	switch self {
	case "boolean", "string", "number":
		return value
	case "undefined", "null":
		return nil
	case "array":
		if value == nil {
			return value
		}
		arr := []any{}
		for _, e := range value.([]any) {
			pair := e.(map[string]any)
			arr = append(arr, deepUnserialize(pair["type"].(string), pair["value"]))
		}
		return arr
	case "object":
		if value == nil {
			return value
		}
		obj := map[string]any{}
		for _, e := range value.([]any) {
			var (
				val  = e.([]any)
				pair = val[1].(map[string]any)
			)
			obj[val[0].(string)] = deepUnserialize(pair["type"].(string), pair["value"])
		}
		return obj
	default:
		return value
	}
}

// implemented
// + undefined, null, string, number, boolean, promise, node, array, object, bigint, function, window
// unimplemented
// - regexp, date, symbol, map, set, weakmap, weakset, error, proxy, typedarray, arraybuffer
func (f *Frame) unserialize(value *runtime.RemoteObject) (any, error) {
	if value == nil {
		return nil, errors.New("can't unserialize nil RemoteObject")
	}
	if value.DeepSerializedValue == nil {
		return value.Value, nil
	}

	switch value.DeepSerializedValue.Type {

	case "promise", "function", "weakmap":
		return RemoteObject(value.ObjectId), nil

	case "node":
		switch getNodeType(value.DeepSerializedValue.Value) {
		case nodeTypeElement, nodeTypeDocument:
			return &Node{JsObject: RemoteObject(value.ObjectId), frame: f}, nil
		default:
			return nil, errors.New("unsupported type of node")
		}

	case "nodelist":
		/* It returns the head of linked nodes list (1th element), not array */
		if value.Description == "NodeList(0)" {
			return nil, nil
		}
		return f.requestNodeList(value.ObjectId)

	default:
		return deepUnserialize(value.DeepSerializedValue.Type, value.DeepSerializedValue.Value), nil
	}
}

func (f *Frame) requestNodeList(objectId runtime.RemoteObjectId) (*NodeList, error) {
	descriptor, err := f.getProperties(RemoteObject(objectId), true, false, false, false)
	if err != nil {
		return nil, err
	}
	var i = 0
	nodeList := &NodeList{}
	for _, d := range descriptor.Result {
		if d.Enumerable {
			i++
			n := &Node{
				JsObject:    RemoteObject(d.Value.ObjectId),
				cssSelector: d.Value.Description + fmt.Sprintf("(%d)", i),
				frame:       f,
			}
			nodeList.Nodes = append(nodeList.Nodes, n)
		}
	}
	return nodeList, nil
}

func (f Frame) toCallArgument(args ...any) (arguments []*runtime.CallArgument) {
	for _, arg := range args {
		callArg := runtime.CallArgument{}
		switch a := arg.(type) {
		case JsObject:
			callArg.ObjectId = a.ObjectID()
		default:
			callArg.Value = a
		}
		arguments = append(arguments, &callArg)
	}
	return
}

func (f Frame) evaluate(expression string, awaitPromise bool) (any, error) {
	var uid = f.executionContextID()
	if uid == "" {
		return nil, ErrExecutionContextDestroyed
	}
	value, err := runtime.Evaluate(f, runtime.EvaluateArgs{
		Expression:            expression,
		IncludeCommandLineAPI: true,
		UniqueContextId:       uid,
		AwaitPromise:          awaitPromise,
		Timeout:               runtime.TimeDelta(f.session.timeout.Milliseconds()),
		SerializationOptions: &runtime.SerializationOptions{
			Serialization: "deep",
		},
	})
	if err != nil {
		return nil, err
	}
	if err = toDOMException(value.ExceptionDetails); err != nil {
		return nil, err
	}
	return f.unserialize(value.Result)
}

func (f Frame) AwaitPromise(promise JsObject) (any, error) {
	value, err := runtime.AwaitPromise(f, runtime.AwaitPromiseArgs{
		PromiseObjectId: promise.ObjectID(),
		ReturnByValue:   true,
		GeneratePreview: false,
	})
	if err != nil {
		return nil, err
	}
	if err = toDOMException(value.ExceptionDetails); err != nil {
		return nil, err
	}
	return f.unserialize(value.Result)
}

func (f Frame) callFunctionOn(self JsObject, function string, awaitPromise bool, args ...any) (any, error) {
	var uid = f.executionContextID()
	if uid == "" {
		return nil, ErrExecutionContextDestroyed
	}
	value, err := runtime.CallFunctionOn(f, runtime.CallFunctionOnArgs{
		FunctionDeclaration: function,
		ObjectId:            self.ObjectID(),
		AwaitPromise:        awaitPromise,
		Arguments:           f.toCallArgument(args...),
		UniqueContextId:     uid,
		SerializationOptions: &runtime.SerializationOptions{
			Serialization: "deep",
		},
	})
	if err != nil {
		return nil, err
	}
	if err = toDOMException(value.ExceptionDetails); err != nil {
		return nil, err
	}
	return f.unserialize(value.Result)
}

func (f Frame) getProperties(self JsObject, ownProperties, accessorPropertiesOnly, generatePreview, nonIndexedPropertiesOnly bool) (*runtime.GetPropertiesVal, error) {
	value, err := runtime.GetProperties(f, runtime.GetPropertiesArgs{
		ObjectId:                 self.ObjectID(),
		OwnProperties:            ownProperties,
		AccessorPropertiesOnly:   accessorPropertiesOnly,
		GeneratePreview:          generatePreview,
		NonIndexedPropertiesOnly: nonIndexedPropertiesOnly,
	})
	if err != nil {
		return nil, err
	}
	if err = toDOMException(value.ExceptionDetails); err != nil {
		return nil, err
	}
	return value, nil
}

func (f Frame) describeNode(self JsObject) (*dom.Node, error) {
	value, err := dom.DescribeNode(f, dom.DescribeNodeArgs{
		ObjectId: self.ObjectID(),
	})
	if err != nil {
		return nil, err
	}
	return value.Node, nil
}

func toDOMException(value *runtime.ExceptionDetails) error {
	if value != nil && value.Exception != nil {
		return DOMException(value.Exception.Description)
	}
	return nil
}
