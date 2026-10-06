package v8go

// InternalRetainedValueCount is exported for testing only.
func (i *Isolate) InternalRetainedValueCount() int {
	return i.internalRetainedValueCount()
}

// InternalUnboundScriptCount is exported for testing only.
func (i *Isolate) InternalUnboundScriptCount() int {
	return i.internalUnboundScriptCount()
}
