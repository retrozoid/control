package control

import (
	"errors"

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

type remoteObjectId runtime.RemoteObjectId

func (r remoteObjectId) ObjectID() runtime.RemoteObjectId {
	return runtime.RemoteObjectId(r)
}

type node struct {
	JsObject
	frame Frame
}

type Optional[T any] struct {
	Value T
	Err   error
}

type OptionalNode Optional[node]
type OptionalNodes Optional[[]node]

func toOptionalNode(value any, err error) OptionalNode {
	if err != nil {
		return OptionalNode{Err: err}
	}
	if value == nil {
		return OptionalNode{Err: ErrElementNotFound}
	}
	if n, ok := value.(node); ok {
		return OptionalNode{Value: n}
	}
	return OptionalNode{Err: ErrElementIsNotNode}
}

func (e OptionalNode) ObjectID() runtime.RemoteObjectId {
	if e.Err == nil {
		return e.Value.ObjectID()
	}
	return ""
}

func (e OptionalNode) Call(method string, send, recv interface{}) error {
	if e.Err != nil {
		return e.Err
	}
	return e.Value.frame.Call(method, send, recv)
}

func (e OptionalNode) eval(function string, args ...any) (any, error) {
	if e.Err != nil {
		return nil, e.Err
	}
	return e.Value.frame.callFunctionOn(e, function, true, args...)
}

func (e OptionalNode) asyncEval(function string, args ...any) (JsObject, error) {
	if e.Err != nil {
		return nil, e.Err
	}
	value, err := e.Value.frame.callFunctionOn(e, function, false, args...)
	if err != nil {
		return nil, err
	}
	return value.(JsObject), nil
}

func getNodeType(deepSerializedValue any) nodeType {
	return nodeType(deepSerializedValue.(map[string]any)["nodeType"].(float64))
}

type evalValue struct {
	Result           *runtime.RemoteObject
	ExceptionDetails *runtime.ExceptionDetails
}

func (f Frame) unserialize(value evalValue) (any, error) {
	if value.ExceptionDetails != nil {
		return nil, DOMException(value.ExceptionDetails.Exception.Description)
	}
	if value.Result.DeepSerializedValue == nil {
		return value.Result.Value, nil
	}
	deepSerializedValue := value.Result.DeepSerializedValue
	switch deepSerializedValue.Type {
	case "undefined", "null", "string", "number", "boolean":
		return deepSerializedValue.Value, nil
	case "promise":
		return remoteObjectId(value.Result.ObjectId), nil
	case "node":
		switch getNodeType(deepSerializedValue.Value) {
		case nodeTypeElement, nodeTypeDocument:
			return node{
				JsObject: remoteObjectId(value.Result.ObjectId),
				frame:    f,
			}, nil
		default:
			return nil, errors.New("unsupported type of node")
		}
	default:
		return value.Result.Value, nil
	}
}

// undefined, null, string, number, boolean, promise, node
//bigint, regexp, date, symbol, array, object, function, map, set, weakmap, weakset, error, proxy, typedarray, arraybuffer, window

func (f Frame) Evaluate(expression string, awaitPromise bool) (any, error) {
	var uid = f.executionContextID()
	if uid == "" {
		return nil, ErrExecutionContextDestroyed
	}
	value, err := runtime.Evaluate(f, runtime.EvaluateArgs{
		Expression:            expression,
		IncludeCommandLineAPI: true,
		UniqueContextId:       uid,
		AwaitPromise:          awaitPromise,
		Timeout:               runtime.TimeDelta(f.session.Timeout.Milliseconds()),
		SerializationOptions: &runtime.SerializationOptions{
			Serialization: "deep",
		},
	})
	if err != nil {
		return nil, err
	}
	return f.unserialize(evalValue{
		Result:           value.Result,
		ExceptionDetails: value.ExceptionDetails,
	})
}

func (f Frame) awaitPromise(promise JsObject) (any, error) {
	value, err := runtime.AwaitPromise(f, runtime.AwaitPromiseArgs{
		PromiseObjectId: promise.ObjectID(),
		ReturnByValue:   false,
		GeneratePreview: true,
	})
	if err != nil {
		return nil, err
	}
	return f.unserialize(evalValue{
		Result:           value.Result,
		ExceptionDetails: value.ExceptionDetails,
	})
}

func (f Frame) eval(node JsObject, function string, args ...any) (any, error) {
	return f.callFunctionOn(node, function, true, args...)
}

func (f Frame) asyncEval(node JsObject, function string, args ...any) (JsObject, error) {
	value, err := f.callFunctionOn(node, function, false, args...)
	if err != nil {
		return nil, err
	}
	return value.(JsObject), nil
}

func (f Frame) callFunctionOn(this JsObject, function string, awaitPromise bool, args ...any) (any, error) {
	var arguments []*runtime.CallArgument
	for _, arg := range args {
		callArg := &runtime.CallArgument{}
		switch a := arg.(type) {
		case JsObject:
			callArg.ObjectId = a.ObjectID()
		default:
			callArg.Value = a
		}
		arguments = append(arguments, callArg)
	}
	value, err := runtime.CallFunctionOn(f, runtime.CallFunctionOnArgs{
		FunctionDeclaration: function,
		ObjectId:            this.ObjectID(),
		AwaitPromise:        awaitPromise,
		Arguments:           arguments,
		SerializationOptions: &runtime.SerializationOptions{
			Serialization: "deep",
		},
	})
	if err != nil {
		return nil, err
	}
	return f.unserialize(evalValue{
		Result:           value.Result,
		ExceptionDetails: value.ExceptionDetails,
	})
}
