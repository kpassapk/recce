package recce

import (
	"bytes"
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"net/http"
	"strconv"
	"testing"
)

type Task struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

var tasks = []Task{
	{ID: 1, Title: "First task"},
}

func getTasks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}

func getTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	params := mux.Vars(r)
	id, err := strconv.Atoi(params["id"])

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid task ID"})
		return
	}

	for _, task := range tasks {
		if task.ID == id {
			json.NewEncoder(w).Encode(task)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(map[string]string{"error": "Task not found"})
}

func createTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var task Task
	_ = json.NewDecoder(r.Body).Decode(&task)

	task.ID = len(tasks) + 1
	tasks = append(tasks, task)

	json.NewEncoder(w).Encode(task)
}

func SetupRouter() *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/tasks", getTasks).Methods("GET")
	r.HandleFunc("/tasks/{id}", getTask).Methods("GET")
	r.HandleFunc("/tasks", createTask).Methods("POST")
	return r
}

func TestRecce(t *testing.T) {
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
			a := assert.New(t)
			rec := Start(n+1, tt.name, a, WithGroup("tasks/create"))
			defer rec.Finish()
			rec.NewRequest("POST", "/tasks", bytes.NewBuffer([]byte(tt.input)))
			res := rec.SendRequest(r.ServeHTTP)
			a.Equal(tt.status, res.StatusCode)
		})
	}
}
