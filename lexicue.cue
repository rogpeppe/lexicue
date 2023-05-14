package lexicue

#Doc: {
	lexicon!: 1
	defs:     or([
			for def in _#defs {
			def
		},
	])
}

_#defs: {
	procedure: {
		_lexicon!: "procedure"
		input?:    #xrpcBody
		output?:   #xrpcBody
		errors?: [... #xrpcError]
	}

	query: {
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
		#blob
	}

	image: {
		_lexicon!: "image"
		#image
	}

	video: {
		_lexicon!: "video"
		#video
	}

	audio: {
		_lexicon: "audio"
		#audio
	}

	token: {
		string
		_lexicon: "token"
	}

	record: {
		_lexicon: "record"
		key?:     string
		record!: {...}
	}

	subscription: {
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
	$link!: =~"^Qm[1-9A-HJ-NP-Za-km-z]{44}|[\u00000fFbBcCvVtTkKzZmuMU]$"
}

#subscriptionMessage: {
	schema!: _
}

#blob: {
	$type!:    "blob"
	ref!:      #cidLink
	mimeType!: string
	size!:     uint
} | #legacyBlob

#legacyBlob: {
	$type?:    !="blob"
	cid!:      string
	mimeType!: string
}

#image: {
	mimeType!: string
	size!: int
	width!: number
	height!: number
}

#video: {
	mimeType!: string
	size!: int
	width!: number
	height!: number
	length!: number
}

#audio: {
	mimeType!: string
	size!: int
	length!: number
}
