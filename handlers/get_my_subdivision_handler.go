package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	jwt_service "todo/JWT"
)

type User_sub struct {
	Balance  string `json:"balance"`
	ID       int    `json:"id"`
	Role     string `json:"role"`
	Username string `json:"username"`
}

type Subdivision_my struct {
	ID              int           `json:"id"`
	SubdivisionName string        `json:"subdivision_name"`
	Owner           User_sub      `json:"owner"`
	Employers       []Employer    `json:"employers"`
	Tasks           []interface{} `json:"tasks"`
}

type Employer struct {
	Username string `json:"username"`
	Balance  string `json:"balance"`
	Role     string `json:"role"`
}

func GetMySubdivision(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	authorizationHeader := r.Header.Get("Authorization")
	if authorizationHeader == "" {
		http.Error(w, "Authorization header is missing", http.StatusUnauthorized)
		return
	}
	authHeaderParts := strings.Split(authorizationHeader, " ")
	if len(authHeaderParts) != 2 || authHeaderParts[0] != "Bearer" {
		http.Error(w, "Invalid Authorization header format", http.StatusUnauthorized)
		return
	}
	tokenString := authHeaderParts[1]
	userID, err := jwt_service.ParseJWT(tokenString)
	if err != nil {
		http.Error(w, "Invalid JWT token", http.StatusUnauthorized)
		return
	}
	rows, err := db.Query(`
		SELECT tasks.id, tasks.created_at, tasks.description, tasks.title, users.subdivision, users.username
		FROM tasks
		INNER JOIN users ON tasks.userid = users.id
		WHERE users.id = $1 OR users.subdivision = (SELECT subdivision FROM users WHERE id = $1)
		ORDER BY tasks.created_at DESC`, userID)
	if err != nil {
		http.Error(w, "Failed to get tasks", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	tasks := []interface{}{}
	for rows.Next() {
		var taskID, taskCreatedAt, taskDescription, taskTitle, taskSubdivision, taskUsername interface{}
		err := rows.Scan(&taskID, &taskCreatedAt, &taskDescription, &taskTitle, &taskSubdivision, &taskUsername)
		if err != nil {
			http.Error(w, "Failed to scan task", http.StatusInternalServerError)
			return
		}
		task := map[string]interface{}{
			"id":          taskID,
			"created_at":  taskCreatedAt,
			"description": taskDescription,
			"title":       taskTitle,
			"subdivision": taskSubdivision,
			"username":    taskUsername,
		}
		tasks = append(tasks, task)
	}
	subdivisionID := 0
	err = db.QueryRow("SELECT subdivision FROM users WHERE id = $1", userID).Scan(&subdivisionID)
	if err != nil {
		http.Error(w, "Failed to get subdivision", http.StatusInternalServerError)
		return
	}
	subdivisionInfo := struct {
		Name  string   `json:"name"`
		Owner User_sub `json:"owner"`
	}{}
	err = db.QueryRow("SELECT name, balance, role, id, username FROM subdivisions JOIN users ON subdivisions.owner = users.id WHERE subdivision_id = $1", subdivisionID).Scan(&subdivisionInfo.Name, &subdivisionInfo.Owner.Balance, &subdivisionInfo.Owner.Role, &subdivisionInfo.Owner.ID, &subdivisionInfo.Owner.Username)
	if err != nil {
		http.Error(w, "Failed to get subdivision info", http.StatusInternalServerError)
		return
	}
	subdivisionEmployers, err := db.Query("SELECT username, balance, role FROM users WHERE subdivision = $1", subdivisionID)
	if err != nil {
		http.Error(w, "Failed to get subdivision employers", http.StatusInternalServerError)
		return
	}
	defer subdivisionEmployers.Close()
	employers := []Employer{}
	for subdivisionEmployers.Next() {
		var username, balance, role string
		err := subdivisionEmployers.Scan(&username, &balance, &role)
		if err != nil {
			http.Error(w, "Failed to scan employer", http.StatusInternalServerError)
			return
		}
		employer := Employer{
			Username: username,
			Balance:  balance,
			Role:     role,
		}
		employers = append(employers, employer)
	}
	data := map[string]interface{}{
		"subdivision": Subdivision_my{
			ID:              subdivisionID,
			SubdivisionName: subdivisionInfo.Name,
			Owner:           subdivisionInfo.Owner,
			Employers:       employers,
			Tasks:           tasks,
		},
	}
	jsonResp, err := json.Marshal(data)
	if err != nil {
		http.Error(w, "Failed to marshal JSON response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonResp)
}
