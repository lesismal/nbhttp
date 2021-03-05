package nbhttp

const (
	// state: RequestLine
	stateMethodBefore int8 = iota
	stateMethod

	statePathBefore
	statePath
	stateProtoBefore
	stateProto
	stateProtoLF
	stateClientProtoBefore
	stateClientProto
	stateStatusCodeBefore
	stateStatusCode
	stateStatusBefore
	stateStatus
	stateStatusLF

	// state: Header
	stateHeaderKeyBefore
	stateHeaderValueLF
	stateHeaderKey

	stateHeaderValueBefore
	stateHeaderValue

	// state: Body ContentLength
	stateBodyContentLength

	// state: Body Chunk
	stateHeaderOverLF
	stateBodyChunkSizeBlankLine
	stateBodyChunkSizeBefore
	stateBodyChunkSize
	stateBodyChunkSizeLF
	stateBodyChunkData
	stateBodyChunkDataCR
	stateBodyChunkDataLF

	// state: Body Trailer
	stateBodyTrailerHeaderValueLF
	stateBodyTrailerHeaderKeyBefore
	stateBodyTrailerHeaderKey
	stateBodyTrailerHeaderValueBefore
	stateBodyTrailerHeaderValue

	// state: Body CRLF
	stateTailCR
	stateTailLF
)
