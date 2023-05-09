package lexicon

#Doc: {
	lexicon!: 1
	defs:     or([
			for def in _#defs {
			def
		},
	])
}

_#defs: {
	xrpcProcedure: {
		_lexicon!: "procedure"
		input?:    #xrpcBody
		output?:   #xrpcBody
		errors?: [... #xrpcError]
	}

	xrpcQuery: {
		_lexicon!: "query"
		parameters?: {...}
		output?: #xrpcBody
		errors?: [... #xrpcError]
	}

	cidLink: {
		_lexicon!: "cidLink"
		#cidLink
	}

	blob: {
		_lexicon!: "blob"
		accept?: [... string]
		maxSize?: int
	}

	image: {
		_lexicon!: "image"
		accept?: [... string]
		maxSize?:   int
		maxWidth?:  number
		maxHeight?: number
	}

	video: {
		_lexicon!: "image"
		accept?: [... string]
		maxSize?:   int
		maxWidth?:  number
		maxHeight?: number
		maxLength?: number
	}

	audio!: {
		_lexicon: "audio"
		accept?: [... string]
		maxSize?:   int
		maxLength?: number
	}

	token!: {
		token!:   string
		_lexicon: "token"
	}

	record!: {
		_lexicon: "record"
		key?:     string
		record!: {...}
	}

	subscription!: {
		_lexicon: "subscription"
		parameters!: {...}
		// TODO should we just fold the schema directly into the message field
		// instead of using the #subscriptionMessage indirection?
		message?: #subscriptionMessage
		errors?: [... #xrpcError]
	}
}

for name, def in _#defs {
	(name): def & {
		_lexicon: _
	}
}

#xrpcBody: {
	description?: string
	// The original seemed to allow an array of string for encoding:
	// encoding!:    string | [... string]
	// but in practice that doesn't seem to happen.
	encoding!: string
	schema?:   _
}

#xrpcError: {
	name!:        string
	description!: string
}

#cidLink: {
	$link!: string
}

#subscriptionMessage: {
	schema!: _
}
