package httpx

type Headers map[string]string

type HeaderValueSetter interface {
	Value(value string) Headers
	IfNotEmpty(value string) Headers
	OrDefault(value, defaultValue string) Headers
}

func NewHeaders() Headers {
	return make(Headers)
}

func (h Headers) All() map[string]string {
	return h
}

func (h Headers) Accept() HeaderValueSetter {
	return h.Set("Accept")
}

func (h Headers) AcceptJSON() Headers {
	return h.Accept().Value("application/json")
}

func (h Headers) Authorization() HeaderValueSetter {
	return h.Set("Authorization")
}

func (h Headers) AuthorizationBearer() HeaderValueSetter {
	return setter(h, "Authorization").withValuePrefix("Bearer ")
}

func (h Headers) ContentType() HeaderValueSetter {
	return h.Set("Content-Type")
}

func (h Headers) ContentTypeJSON() Headers {
	return h.ContentType().Value("application/json")
}

func (h Headers) UserAgent() HeaderValueSetter {
	return h.Set("User-Agent")
}

func (h Headers) Set(key string) HeaderValueSetter {
	return setter(h, key)
}

type headersSetter struct {
	key         string
	headers     Headers
	valuePrefix string
}

func setter(headers Headers, key string) *headersSetter {
	return &headersSetter{
		key:     key,
		headers: headers,
	}
}

func (s *headersSetter) Value(value string) Headers {
	s.headers[s.key] = s.valuePrefix + value
	return s.headers
}

func (s *headersSetter) IfNotEmpty(value string) Headers {
	if value != "" {
		s.headers[s.key] = s.valuePrefix + value
	}
	return s.headers
}

func (s *headersSetter) OrDefault(value, defaultValue string) Headers {
	if value == "" {
		s.headers[s.key] = s.valuePrefix + defaultValue
	} else {
		s.headers[s.key] = s.valuePrefix + value
	}
	return s.headers
}

func (s *headersSetter) withValuePrefix(valuePrefix string) *headersSetter {
	s.valuePrefix = valuePrefix
	return s
}
