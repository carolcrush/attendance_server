package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"freee/db"
	"log"
	"net/http"
	"regexp"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type user struct {
	Id       string `json:"id"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

type latestAttendance struct {
	Id    string  `json:"id"`
	Start string  `json:"start"`
	End   *string `json:"end"`
}

type insertAttendanceParams struct {
	UserId   string `json:"userId"`
	Kind     string `json:"kind"`
	Time     string `json:"time"`
	Password string `json:"password"`
}

type totalAttendance struct {
	Id     string  `json:"id"`
	UserId string  `json:"userId"`
	Name   string  `json:"name"`
	Start  string  `json:"start"`
	End    *string `json:"end"`
}

func hashedUserPassword(password string) string {
	passwordBytes := []byte(password)
	hashedPassword := sha256.Sum256(passwordBytes)
	userPassword := hex.EncodeToString(hashedPassword[:])
	return userPassword
}

func isValidName(name string) error {
	if len(name) < 3 || len(name) > 255 || !regexp.MustCompile("^[a-zA-Z\\p{Han}]+$").MatchString(name) {
		return errors.New("ERROR")
	}
	return nil
}

func isValidPassword(password string) error {
	if len(password) < 8 || len(password) > 255 || !regexp.MustCompile("^[a-zA-Z0-9]+$").MatchString(password) {
		return errors.New("ERROR")
	}
	return nil
}

func main() {
	e := echo.New()
	e.Use(middleware.CORS())
	e.GET("/user", getUsers)
	e.POST("/user", createUser)
	e.POST("/attendance", createAttendance)
	e.GET("/admin", getTotalAttendance)

	e.Logger.Fatal(e.Start(":8080"))
}

func createUser(c echo.Context) error {
	var user user
	if err := c.Bind(&user); err != nil {
		return c.JSON(http.StatusBadRequest, "Pamameter is invalid")
	}

	e1 := isValidName(user.Name)
	if e1 != nil {
		log.Println("err", e1)
		return c.JSON(http.StatusBadRequest, "Name is invalid")
	}

	e2 := isValidPassword(user.Password)
	if e2 != nil {
		log.Println("err", e2)
		return c.JSON(http.StatusBadRequest, "Password is invalid")
	}

	user.Password = hashedUserPassword(user.Password)
	err := insertUser(&user)
	if err != nil {
		return c.JSON(http.StatusBadRequest, "ID is invalid")
	}
	return c.JSON(http.StatusOK, "OK")
}

func selectUsers() ([]user, error) {
	rows, err := db.Conn.Query("SELECT id, name FROM user")
	if err != nil {
		log.Println("Error querying user", err)
		return nil, err
	}
	var users []user
	for rows.Next() {
		var user user
		if err := rows.Scan(&user.Id, &user.Name); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}

func selectUsersById(userId string) (string, error) {
	row := db.Conn.QueryRow("SELECT password FROM user WHERE id=?", userId)
	var password string
	if err := row.Scan(&password); err != nil {
		if err == sql.ErrNoRows {
			return "", err
		}
		return "", err
	}
	return password, nil
}

func getUsers(c echo.Context) error {
	users, err := selectUsers()
	if err != nil {
		return c.JSON(http.StatusBadRequest, "Users not found")
	}
	return c.JSON(http.StatusOK, users)
}

func createAttendance(c echo.Context) error {
	var insertAttendanceParams insertAttendanceParams
	if err := c.Bind(&insertAttendanceParams); err != nil {
		return c.JSON(http.StatusBadRequest, "Attendance parameters not found")
	}

	userPassword, e := selectUsersById(insertAttendanceParams.UserId)
	if userPassword == "" {
		return c.JSON(http.StatusBadRequest, "Password not found")
	}
	if e != nil {
		return c.JSON(http.StatusBadRequest, "Users not found")
	}

	attendancePassword := hashedUserPassword(insertAttendanceParams.Password)
	if userPassword != attendancePassword {
		return c.JSON(http.StatusBadRequest, "Incorrect password")
	}

	switch insertAttendanceParams.Kind {
	case "start":
		err := insertAttendance(insertAttendanceParams.UserId, insertAttendanceParams.Time)
		if err != nil {
			return c.JSON(http.StatusBadRequest, "Failed to insert attendance")
		}
		return c.JSON(http.StatusOK, "OK")

	case "end":
		latestAttendance, err := selectLatestAttendance(insertAttendanceParams.UserId)
		if err != nil {
			return c.JSON(http.StatusBadRequest, "Latest attendance not found")
		}

		if len(latestAttendance.Start) > 0 && latestAttendance.End == nil {
			err := updateAttendance(latestAttendance.Id, insertAttendanceParams.Time)
			if err != nil {
				return c.JSON(http.StatusBadRequest, "Failed to update attendance")
			}
			return c.JSON(http.StatusOK, "OK")
		}

		return c.JSON(http.StatusBadRequest, "Start time not found")

	default:
		return c.JSON(http.StatusBadRequest, "Invalid attendance kind")
	}
}

func insertUser(user *user) error {
	if _, err := db.Conn.Exec(
		"INSERT INTO user (id, name, password) values (?, ?, ?)",
		user.Id,
		user.Name,
		user.Password,
	); err != nil {
		return err
	}
	return nil
}

func insertAttendance(userId string, time string) error {
	if _, err := db.Conn.Exec(
		"INSERT INTO attendance (user_id, start) values (?, ?)",
		userId,
		time,
	); err != nil {
		log.Println("err", err)
		return err
	}
	return nil
}

func selectLatestAttendance(userId string) (*latestAttendance, error) {
	row := db.Conn.QueryRow("SELECT id, start, end FROM attendance WHERE user_id=? ORDER BY id DESC LIMIT 1", userId)
	var latestAttendance latestAttendance
	if err := row.Scan(&latestAttendance.Id, &latestAttendance.Start, &latestAttendance.End); err != nil {
		if err == sql.ErrNoRows {
			log.Println("No rows found for user:", userId)
			return nil, err
		}
		log.Println("Error scanning row:", err)
		return nil, err
	}
	return &latestAttendance, nil
}

func updateAttendance(id string, time string) error {
	if _, err := db.Conn.Exec(
		"UPDATE attendance SET end=? WHERE id=?",
		time,
		id,
	); err != nil {
		log.Println("err", err)
		return err
	}
	return nil
}

func getTotalAttendance(c echo.Context) error {
	totalAttendances, err := selectTotalAttendance()
	if err != nil {
		return c.JSON(http.StatusBadRequest, "Total attendances not found")
	}
	return c.JSON(http.StatusOK, totalAttendances)
}

func selectTotalAttendance() ([]totalAttendance, error) {
	rows, err := db.Conn.Query(`
            SELECT attendance.*, user.name
            FROM attendance
            JOIN user ON attendance.user_id = user.id
        `)
	if err != nil {
		log.Println("err", err)
		return nil, err
	}

	var totalAttendances []totalAttendance
	for rows.Next() {
		var attendance totalAttendance
		if err := rows.Scan(&attendance.Id, &attendance.UserId, &attendance.Start, &attendance.End, &attendance.Name); err != nil {
			log.Println("err", err)
			return nil, err
		}
		totalAttendances = append(totalAttendances, attendance)
	}
	return totalAttendances, nil
}
