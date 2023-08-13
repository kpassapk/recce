# Recce: A Testify-based test recorder compatible with REST Client

*Recce* (ReKi) [V]: to visit [a] place in order to become familiar with it.
- Collins

Recce is a simple test reorder for testing APIs in Go in the early stages, when
things are still in flux and you want visibility. It saves `httptest`
interactions in a format compatible with [REST Client][restclient]

[restclient]: https://marketplace.visualstudio.com/items?itemName=humao.rest-client

## Install


``` sh
go get github.com/kpassapk/recce
```

## Use

Suppose we have an API endpoint `POST /tasks`.

``` sh
import (
	"bytes"
	"net/http"
	"testing"
    "github.com/kpassapk/recce"
)


func TestCreateTask(t *testing.T) {
	r := SetupRouter()

	tests := []struct {
		name   string
		input  string
		status int
		output Task
	}{
		{
			name:   "valid task",
			input:  `{"title": "Test Task"}`,
			status: http.StatusOK,
		},
	}

	for n, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := recce.Record(t, "tasks/create", n+1)
			defer rec.Finish()
			rec.SetRequest("POST", "/tasks", bytes.NewBuffer([]byte(tt.input)))
			rec.SendRequest(r.ServeHTTP)

			tester.CheckStatus(tt.status)
		})
	}
}
```

will record HTTP interactions in a `.rest` file

``` sh
recordings
└── tasks
    └──── create
         └── example1.rest
```

**example1.rest**
``` sh
POST localhost:8080/tasks HTTP/1.1

{
    "title": "Test Task"
}
```


If you have [REST Client][restclient] installed, you can re-run the same request that your test just ran. 

## Responses

By default, responses are saved to a text file with a `.resp` format. 

**example1.resp**
```
// Automatically generated. Do not edit.
// File generated at 13 August 2023 18:30:59
// ------------------------------------------------------------
POST /tasks HTTP/1.1

HTTP/1.1 200 OK
Content-Type: application/json

{"id":2,"title":"Test Task"}

```

The `localhost:8080` can be modified to the default port your application uses in development using the `WithHost()` and `WithPort()` options.
