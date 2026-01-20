package grit

type DocEntry struct {
	Method    string
	Path      string
	Summary   string
	Protected bool
	Body      map[string]interface{}
}

// key = METHOD:PATH
var docsRegistry = map[string]DocEntry{}

func docKey(method, path string) string {
	return method + ":" + path
}

// auto registration (called from router)
func registerDocs(method, path string) {
	key := docKey(method, path)

	if _, exists := docsRegistry[key]; exists {
		return
	}

	docsRegistry[key] = DocEntry{
		Method:  method,
		Path:    path,
		Summary: method + " " + path,
	}
}

// optional explicit override
func registerDoc(d DocEntry) {
	key := docKey(d.Method, d.Path)
	docsRegistry[key] = d
}

func getDocs() []DocEntry {
	out := make([]DocEntry, 0, len(docsRegistry))
	for _, d := range docsRegistry {
		out = append(out, d)
	}
	return out
}
