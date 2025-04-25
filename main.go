package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"sort"
	"strconv"
	"strings"
	"time"
)

type Priority string

const (
	Low    Priority = "low"
	Medium Priority = "medium"
	High   Priority = "high"
)

type Task struct {
	ID          int       `json:"id"`
	Description string    `json:"description"`
	DueDate     time.Time `json:"due_date"`
	Priority    Priority  `json:"priority"`
	Completed   bool      `json:"completed"`
}

type TodoList struct {
	Tasks []Task
	File  string
}

func NewTodoList(filename string) *TodoList {
	return &TodoList{File: filename}
}

func (t *TodoList) SaveTasks() error {
	data, err := json.MarshalIndent(t.Tasks, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(t.File, data, 0644)
}

func (t *TodoList) LoadTasks() error {
	data, err := ioutil.ReadFile(t.File)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &t.Tasks)
}

func (t *TodoList) AddTask(description string, dueDate time.Time, priority Priority) {
	id := 1
	if len(t.Tasks) > 0 {
		id = t.Tasks[len(t.Tasks)-1].ID + 1
	}

	task := Task{
		ID:          id,
		Description: description,
		DueDate:     dueDate,
		Priority:    priority,
		Completed:   false,
	}

	t.Tasks = append(t.Tasks, task)
}

func (t *TodoList) ListTasks(sortBy string) {
	if len(t.Tasks) == 0 {
		fmt.Println("No tasks found.")
		return
	}

	tasks := make([]Task, len(t.Tasks))
	copy(tasks, t.Tasks)

	switch sortBy {
	case "date":
		sort.Slice(tasks, func(i, j int) bool {
			return tasks[i].DueDate.Before(tasks[j].DueDate)
		})
	case "priority":
		sort.Slice(tasks, func(i, j int) bool {
			pMap := map[Priority]int{High: 3, Medium: 2, Low: 1}
			return pMap[tasks[i].Priority] > pMap[tasks[j].Priority]
		})
	}

	fmt.Println("\nTasks:")
	fmt.Println("----------------------------------------")
	for _, task := range tasks {
		status := " "
		if task.Completed {
			status = "✓"
		}
		fmt.Printf("[%s] %d. %s\n", status, task.ID, task.Description)
		fmt.Printf("   Due: %s, Priority: %s\n", task.DueDate.Format("2006-01-02"), task.Priority)
		fmt.Println("----------------------------------------")
	}
}

func (t *TodoList) MarkComplete(id int) error {
	for i := range t.Tasks {
		if t.Tasks[i].ID == id {
			t.Tasks[i].Completed = true
			return nil
		}
	}
	return fmt.Errorf("task with ID %d not found", id)
}

func (t *TodoList) DeleteTask(id int) error {
	for i := range t.Tasks {
		if t.Tasks[i].ID == id {
			t.Tasks = append(t.Tasks[:i], t.Tasks[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("task with ID %d not found", id)
}

func (t *TodoList) FilterTasks(priority Priority, completed *bool) []Task {
	var filtered []Task
	for _, task := range t.Tasks {
		if (priority == "" || task.Priority == priority) &&
			(completed == nil || task.Completed == *completed) {
			filtered = append(filtered, task)
		}
	}
	return filtered
}

func main() {
	todoList := NewTodoList("tasks.json")
	if err := todoList.LoadTasks(); err != nil {
		log.Printf("Error loading tasks: %v\n", err)
	}

	// Serve static files
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.ServeFile(w, r, "index.html")
			return
		}
		http.NotFound(w, r)
	})

	// API endpoints
	http.HandleFunc("/api/tasks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode(todoList.Tasks)

		case http.MethodPost:
			var task struct {
				Description string   `json:"description"`
				DueDate     string   `json:"dueDate"`
				Priority    Priority `json:"priority"`
			}

			if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			dueDate, err := time.Parse("2006-01-02", task.DueDate)
			if err != nil {
				http.Error(w, "Invalid date format", http.StatusBadRequest)
				return
			}

			todoList.AddTask(task.Description, dueDate, task.Priority)
			todoList.SaveTasks()
			w.WriteHeader(http.StatusCreated)

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	http.HandleFunc("/api/tasks/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Extract task ID from URL
		pathParts := strings.Split(r.URL.Path, "/")
		if len(pathParts) < 4 {
			http.Error(w, "Invalid task ID", http.StatusBadRequest)
			return
		}

		taskID, err := strconv.Atoi(pathParts[3])
		if err != nil {
			http.Error(w, "Invalid task ID", http.StatusBadRequest)
			return
		}

		// Handle complete endpoint
		if strings.HasSuffix(r.URL.Path, "/complete") {
			if r.Method == http.MethodPost {
				if err := todoList.MarkComplete(taskID); err != nil {
					http.Error(w, err.Error(), http.StatusNotFound)
					return
				}
				todoList.SaveTasks()
				w.WriteHeader(http.StatusNoContent)
			} else {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}

		// Handle delete endpoint
		if r.Method == http.MethodDelete {
			if err := todoList.DeleteTask(taskID); err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			todoList.SaveTasks()
			w.WriteHeader(http.StatusNoContent)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	log.Println("Server starting on http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
	fmt.Println("\nTodo List Application")
	fmt.Println("1. Add Task")
	fmt.Println("2. List Tasks")
	fmt.Println("3. Mark Task as Complete")
	fmt.Println("4. Delete Task")
	fmt.Println("5. Filter Tasks")
	fmt.Println("6. Exit")
	fmt.Print("Choose an option: ")

	var choice string
	fmt.Scanln(&choice)

	switch choice {
	case "1":
		fmt.Print("Enter task description: ")
		var description string
		fmt.Scanln(&description)

		fmt.Print("Enter due date (YYYY-MM-DD): ")
		var dateStr string
		fmt.Scanln(&dateStr)
		dueDate, _ := time.Parse("2006-01-02", dateStr)

		fmt.Print("Enter priority (low/medium/high): ")
		var priority string
		fmt.Scanln(&priority)

		todoList.AddTask(description, dueDate, Priority(strings.ToLower(priority)))
		todoList.SaveTasks()
		fmt.Println("Task added successfully!")

	case "2":
		fmt.Print("Sort by (date/priority/none): ")
		var sortBy string
		fmt.Scanln(&sortBy)
		todoList.ListTasks(sortBy)

	case "3":
		fmt.Print("Enter task ID to mark as complete: ")
		var id string
		fmt.Scanln(&id)
		taskID, _ := strconv.Atoi(id)
		if err := todoList.MarkComplete(taskID); err != nil {
			fmt.Printf("Error: %v\n", err)
		} else {
			todoList.SaveTasks()
			fmt.Println("Task marked as complete!")
		}

	case "4":
		fmt.Print("Enter task ID to delete: ")
		var id string
		fmt.Scanln(&id)
		taskID, _ := strconv.Atoi(id)
		if err := todoList.DeleteTask(taskID); err != nil {
			fmt.Printf("Error: %v\n", err)
		} else {
			todoList.SaveTasks()
			fmt.Println("Task deleted successfully!")
		}

	case "5":
		fmt.Print("Filter by priority (low/medium/high/all): ")
		var priority string
		fmt.Scanln(&priority)
		var filterPriority Priority
		if priority != "all" {
			filterPriority = Priority(priority)
		}

		fmt.Print("Filter by status (completed/incomplete/all): ")
		var status string
		fmt.Scanln(&status)
		var completed *bool
		if status != "all" {
			completedValue := status == "completed"
			completed = &completedValue
		}

		filtered := todoList.FilterTasks(filterPriority, completed)
		if len(filtered) == 0 {
			fmt.Println("No tasks found matching the filters.")
		} else {
			fmt.Println("\nFiltered Tasks:")
			fmt.Println("----------------------------------------")
			for _, task := range filtered {
				status := " "
				if task.Completed {
					status = "✓"
				}
				fmt.Printf("[%s] %d. %s\n", status, task.ID, task.Description)
				fmt.Printf("   Due: %s, Priority: %s\n", task.DueDate.Format("2006-01-02"), task.Priority)
				fmt.Println("----------------------------------------")
			}
		}

	case "6":
		fmt.Println("Goodbye!")
		os.Exit(0)

	default:
		fmt.Println("Invalid option. Please try again.")
	}
}
