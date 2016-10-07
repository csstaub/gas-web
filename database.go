package main

type results struct {
	Path    string `dynamo:"path"`
	Time    int64  `dynamo:"time"`
	Error   string `dynamo:"error"`
	Results string `dynamo:"results"`
}

type database interface {
	store(res results)
	load(path string) (results, bool)
}

type memoryDatabase struct {
	data map[string]results
}

func newMemoryDatabase() database {
	return memoryDatabase{map[string]results{}}
}

func (m memoryDatabase) store(res results) {
	m.data[res.Path] = res
}

func (m memoryDatabase) load(path string) (results, bool) {
	r, ok := m.data[path]
	return r, ok
}
