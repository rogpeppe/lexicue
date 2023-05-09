// derived from the doc in https://atproto.com/specs/lexicon
// Updated so it validates all the schemas at https://github.com/bluesky-social/atproto/tree/aabbf43a7f86b37cefbba614d408534b59f59525/lexicons.

package lexicon

#LexiconDoc: {
	lexicon!:     1
	id!:          string
	revision?:    _|_ // Not apparently used; originally number.
	description?: string
	defs!: [string]: #LexUserType | #LexType
}

#LexType: #LexArray |
	#LexObject |
	#LexPrimitive |
	#LexUnion |
	#LexBlob |
	#LexCIDLink |
	#LexRef

	// The original document allowed an array of LexRef here, but no documents
	// use that functionality and it's not clear to me what it would mean, so I've left
	// it out for now.
	// | [...#LexRef]

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
	ref!:  string
}

#LexUnion: {
	type!: "union"
	refs!: [... #LexRef]
	closed?: bool // can this be false?
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
	type!:       "query"
	parameters!: #LexParams
	output!:     #LexXrpcBody
	errors?: [... #LexXrpcError]
}

#LexXrpcProcedure: {
	#Common
	type!:   "procedure"
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
	maxSize?: int
}

#LexImage: {
	#Common
	type!: "image"
	accept?: [... string]
	maxSize?:   int
	maxWidth?:  number
	maxHeight?: number
}

#LexVideo: {
	#Common
	type!: "video"
	accept?: [... string]
	maxSize?:   int
	maxWidth?:  number
	maxHeight?: number
	maxLength?: number
}

#LexAudio: {
	#Common
	type!: "audio"
	accept?: [... string]
	maxSize?:   int
	maxLength?: number
}

#LexSubscription: {
	#Common
	type!:       "subscription"
	parameters!: #LexParams
	message?:    #LexSubscriptionMessage
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
	type!:      "bytes"
	maxLength?: int
}

#LexUnknown: {
	#Common
	type!: "unknown"
}
