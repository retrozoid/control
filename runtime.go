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

// todo
type JsFunction interface {
	Call(...any) any
}

type remoteObjectId runtime.RemoteObjectId

func (r remoteObjectId) ObjectID() runtime.RemoteObjectId {
	return runtime.RemoteObjectId(r)
}

func getNodeType(deepSerializedValue any) nodeType {
	return nodeType(deepSerializedValue.(map[string]any)["nodeType"].(float64))
}

// implemented
// + undefined, null, string, number, boolean, promise, node, array, object
// unimplemented
// - bigint, regexp, date, symbol, function, map, set, weakmap, weakset, error, proxy, typedarray, arraybuffer, window

func (f *Frame) unserialize(value *runtime.RemoteObject) (any, error) {
	if value == nil {
		return nil, errors.New("can't unserialize nil RemoteObject")
	}
	if value.DeepSerializedValue == nil {
		return value.Value, nil
	}
	deepSerializedValue := value.DeepSerializedValue

	switch deepSerializedValue.Type {

	// primitive types
	case "undefined", "null", "string", "number", "boolean":
		return deepSerializedValue.Value, nil

	case "promise":
		return remoteObjectId(value.ObjectId), nil

	case "node":
		switch getNodeType(deepSerializedValue.Value) {
		case nodeTypeElement, nodeTypeDocument:
			return &Node{JsObject: remoteObjectId(value.ObjectId), frame: f}, nil
		default:
			return nil, errors.New("unsupported type of node")
		}

		/* It returns the head of linked nodes list (1th element), not array */
	case "nodelist":
		if value.Description == "NodeList(0)" {
			return nil, nil
		}
		descriptor, err := f.getProperties(remoteObjectId(value.ObjectId), true, false, false, false)
		if err != nil {
			return nil, err
		}
		var (
			head *Node
			ptr  *Node
			i    = 0
		)
		for _, d := range descriptor.Result {
			if d.Enumerable {
				i++
				n := &Node{
					JsObject:    remoteObjectId(d.Value.ObjectId),
					cssSelector: d.Value.Description + fmt.Sprintf("(%d)", i),
					frame:       f,
				}
				if head == nil {
					head, ptr = n, n
				} else {
					ptr.sibling = n
					ptr = ptr.sibling
				}
			}
		}
		return head, nil

	case "array":
		array := value.DeepSerializedValue.Value.([]any)
		var t = make([]any, len(array))
		for n, a := range array {
			t[n] = a.(map[string]any)["value"]
		}
		return t, nil

	case "object":
		//  [[x map[type:number value:543.8359375]] [y map[type:number value:5211.6328125]] [width map[type:number value:112.3203125]] [height map[type:number value:22.3984375]]]
		return value.DeepSerializedValue.Value.(any), nil

	default:
		return value.Value, nil
	}
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
	value, err := runtime.CallFunctionOn(f, runtime.CallFunctionOnArgs{
		FunctionDeclaration: function,
		ObjectId:            self.ObjectID(),
		AwaitPromise:        awaitPromise,
		Arguments:           f.toCallArgument(args...),
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
