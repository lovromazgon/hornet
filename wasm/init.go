//go:build wasm

package wasm

var handler Handler = HandlerFunc(func(string, []byte) []byte {
	return []byte("no handler set, call wasm.Init() in the plugin code to set a handler")
})

// Handler is the bridge between the WebAssembly exports and the Wasm plugin.
type Handler interface {
	// Handle gets called for every host call to the Wasm plugin.
	Handle(method string, req []byte) (resp []byte)
}

// HandlerFunc is a function type that implements the Handler interface.
// It defines a function that takes a byte slice as input and returns a
// byte slice as output.
type HandlerFunc func(method string, req []byte) (resp []byte)

func (f HandlerFunc) Handle(method string, req []byte) (resp []byte) { return f(method, req) }

// Init needs to be called in an init function in the wasm plugin to initialize
// the wasm call handler.
func Init(h Handler) {
	handler = h
}
