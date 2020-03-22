package main

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo"
)

func addMessage(channelID, userID int64, content string) (int64, error) {
	res, err := db.Exec(
		"INSERT INTO message (channel_id, user_id, content, created_at) VALUES (?, ?, ?, NOW())",
		channelID, userID, content)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

type Message struct {
	ID        int64     `db:"id"`
	ChannelID int64     `db:"channel_id"`
	UserID    int64     `db:"user_id"`
	Content   string    `db:"content"`
	CreatedAt time.Time `db:"created_at"`
}

func queryMessages(chanID, lastID int64) ([]Message, error) {
	msgs := []Message{}
	err := db.Select(&msgs, "SELECT * FROM message WHERE id > ? AND channel_id = ? ORDER BY id DESC LIMIT 100",
		lastID, chanID)
	return msgs, err
}

func postMessage(c echo.Context) error {
	user, err := ensureLogin(c)
	if user == nil {
		return err
	}

	message := c.FormValue("message")
	if message == "" {
		return echo.ErrForbidden
	}

	var chanID int64
	if x, err := strconv.Atoi(c.FormValue("channel_id")); err != nil {
		return echo.ErrForbidden
	} else {
		chanID = int64(x)
	}

	if _, err := addMessage(chanID, user.ID, message); err != nil {
		return err
	}

	return c.NoContent(204)
}

func jsonifyMessage(m Message) (map[string]interface{}, error) {
	u := User{}
	err := db.Get(&u, "SELECT name, display_name, avatar_icon FROM user WHERE id = ?",
		m.UserID)
	if err != nil {
		return nil, err
	}

	r := make(map[string]interface{})
	r["id"] = m.ID
	r["user"] = u
	r["date"] = m.CreatedAt.Format("2006/01/02 15:04:05")
	r["content"] = m.Content
	return r, nil
}

func jsonifyMessages(m []Message) []map[string]interface{} {
	u := []User{}
	var uIds []string = make([]string, 0)
	for i := len(m) - 1; i >= 0; i-- {
		uIds = append(uIds, strconv.Itoa(int(m[i].UserID)))
	}
	log.Printf("DEBUG:uIDs:%v", uIds)

	db.Select(&u, "SELECT id, name, display_name, avatar_icon FROM user WHERE id = IN(?)", strings.Join(uIds, ","))

	log.Printf("DEBUG:u:%v", u)
	response := make([]map[string]interface{}, 0)

	for i := len(m) - 1; i >= 0; i-- {
		var r map[string]interface{}
		r["id"] = m[i].ID
		r["user"] = findUserFromArray(u, uIds[i])
		r["date"] = m[i].CreatedAt.Format("2006/01/02 15:04:05")
		r["content"] = m[i].Content
		response = append(response, r)
	}
	log.Printf("DEBUG:res:%v", response)
	return response
}

func findUserFromArray(users []User, uID string) User {
	uIDInt, _ := strconv.ParseInt(uID, 10, 64)
	for _, v := range users {
		if v.ID == uIDInt {
			return v
		}
	}
	return User{}
}

func getMessage(c echo.Context) error {
	userID := sessUserID(c)
	if userID == 0 {
		return c.NoContent(http.StatusForbidden)
	}

	chanID, err := strconv.ParseInt(c.QueryParam("channel_id"), 10, 64)
	if err != nil {
		return err
	}
	lastID, err := strconv.ParseInt(c.QueryParam("last_message_id"), 10, 64)
	if err != nil {
		return err
	}

	messages, err := queryMessages(chanID, lastID)
	if err != nil {
		return err
	}

	response := jsonifyMessages(messages)
	// response := make([]map[string]interface{}, 0)
	// for i := len(messages) - 1; i >= 0; i-- {
	// 	m := messages[i]
	// 	r, err := jsonifyMessage(m)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	response = append(response, r)
	// }

	if len(messages) > 0 {
		_, err := db.Exec("INSERT INTO haveread (user_id, channel_id, message_id, updated_at, created_at)"+
			" VALUES (?, ?, ?, NOW(), NOW())"+
			" ON DUPLICATE KEY UPDATE message_id = ?, updated_at = NOW()",
			userID, chanID, messages[0].ID, messages[0].ID)
		if err != nil {
			return err
		}
	}

	return c.JSON(http.StatusOK, response)
}
