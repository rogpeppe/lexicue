// derived from the doc in https://atproto.com/specs/lexicon
package lexicon

#LexiconDoc: {
	lexicon!:     1
	id!:          string
	revision?:    number
	description?: string
	defs!: [string]: #LexUserType | #LexType
}

#LexType: #LexArray |
	#LexObject |
	#LexBlob |
	#LexPrimitive |
	#LexUnion |
	#LexCIDLink |
	#LexRef |
	[...#LexRef]

#LexPrimitive: #LexBoolean |
	#LexNumber |
	#LexInteger |
	#LexString |
	#LexBytes |
	#LexUnknown

#LexUserType: #LexXrpcQuery |
	#LexXrpcProcedure |
	#LexRecord |
	#LexToken |
	#LexImage |
	#LexVideo |
	#LexAudio |
	#LexSubscription

#Common: {
	type!:        string
	description?: string
}

#LexRef: string | {
	type!: "ref"
	ref!:   string
}

#LexUnion: {
	type!: "union"
	refs!: [... #LexRef]
	closed?: bool		// can this be false?
}

#LexToken: {
	#Common
	type!: "token"
}

#LexObject: {
	#Common
	type!: "object"
	required?: [... string]
	properties!: [string]: #LexType
	nullable?: [... string]
}

#LexParams: {
	#Common
	type!: "params"
	required?: [... string]
	properties!: [string]: #LexType
}

// database
// =

#LexRecord: {
	#Common
	type!:   "record"
	key?:    string
	record!: #LexObject
}

// XRPC
// =

#LexXrpcQuery: {
	#Common
	type!: "query"
	parameters!: #LexParams
	output!: #LexXrpcBody
	errors?: [... #LexXrpcError]
}

#LexXrpcProcedure: {
	#Common
	type!: "procedure"
	parameters?: [string]: #LexPrimitive
	input?:  #LexXrpcBody
	output?: #LexXrpcBody
	errors?: [... #LexXrpcError]
}

#LexXrpcBody: {
	description?: string
	encoding!:    string | [... string]
	schema!:      #LexType
}

#LexXrpcError: {
	name!:        string
	description?: string
}

#LexCIDLink: {
	#Common
	type!: "cid-link"
}

// blobs
// =

#LexBlob: {
	#Common
	type!: "blob"
	accept?: [... string]
	maxSize?: number
}

#LexImage: {
	#Common
	type!: "image"
	accept?: [... string]
	maxSize?:   number
	maxWidth?:  number
	maxHeight?: number
}

#LexVideo: {
	#Common
	type!: "video"
	accept?: [... string]
	maxSize?:   number
	maxWidth?:  number
	maxHeight?: number
	maxLength?: number
}

#LexAudio: {
	#Common
	type!: "audio"
	accept?: [... string]
	maxSize?:   number
	maxLength?: number
}

#LexSubscription: {
	#Common
	type!: "subscription"
	parameters!: #LexParams
	message?: #LexSubscriptionMessage
	errors?: [... #LexXrpcError]
}

#LexSubscriptionMessage: {
	schema!: #LexType
}

// primitives
// =

#LexArray: {
	#Common
	type!: "array"
	//items:        #LexRef | #LexPrimitive | [... #LexRef]
	// TODO is this actually more restrictive?
	items:      #LexType
	minLength?: int
	maxLength?: int
}

#LexBoolean: {
	#Common
	type!:    "boolean"
	default?: bool
	const?:   bool
}

#LexNumber: {
	#Common
	type!:    "number"
	default?: number
	minimum?: number
	maximum?: number
	enum?: [... number]
	const?: number
}

#LexInteger: {
	#Common
	type!:    "integer"
	default?: number
	minimum?: number
	maximum?: number
	enum?: [... number]
	const?: number
}

#LexString: {
	#Common
	type!:         "string"
	format?:       string // TODO rog
	default?:      string
	minLength?:    int
	maxGraphemes?: int
	maxLength?:    int
	enum?: [... string]
	const?: string
	knownValues?: [... string]
}

#LexBytes: {
	#Common
	type!: "bytes"
	maxLength?: int
}

#LexUnknown: {
	#Common
	type!: "unknown"
}