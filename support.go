package lobster

import "database/sql"
import "log"
import "time"

type TicketMessage struct {
	Id int
	Staff bool
	Message string
	Time time.Time
}

type Ticket struct {
	Id int
	UserId int
	Name string
	Status string
	Time time.Time
	ModifyTime time.Time
	Messages []*TicketMessage
}

func ticketList(db *Database, userId int) []*Ticket {
	return ticketListHelper(db.Query("SELECT id, user_id, name, status, time, modify_time FROM tickets WHERE user_id = ? ORDER BY modify_time DESC", userId))
}
func ticketListActive(db *Database, userId int) []*Ticket {
	return ticketListHelper(db.Query("SELECT id, user_id, name, status, time, modify_time FROM tickets WHERE user_id = ? AND (status = 'open' OR status = 'answered') ORDER BY modify_time DESC", userId))
}
func ticketListAll(db *Database) []*Ticket {
	return ticketListHelper(db.Query("SELECT id, user_id, name, status, time, modify_time FROM tickets ORDER BY FIELD(status, 'open', 'answered', 'closed'), modify_time DESC"))
}
func ticketListHelper(rows *sql.Rows) []*Ticket {
	tickets := make([]*Ticket, 0)
	defer rows.Close()
	for rows.Next() {
		ticket := Ticket{}
		rows.Scan(&ticket.Id, &ticket.UserId, &ticket.Name, &ticket.Status, &ticket.Time, &ticket.ModifyTime)
		tickets = append(tickets, &ticket)
	}
	return tickets
}

func ticketDetails(db *Database, userId int, ticketId int, staff bool) *Ticket {
	var rows *sql.Rows
	if staff {
		rows = db.Query("SELECT id, user_id, name, status, time, modify_time FROM tickets WHERE id = ?", ticketId)
	} else {
		rows = db.Query("SELECT id, user_id, name, status, time, modify_time FROM tickets WHERE user_id = ? AND id = ?", userId, ticketId)
	}
	tickets := ticketListHelper(rows)
	if len(tickets) != 1 {
		return nil
	}
	ticket := tickets[0]

	rows = db.Query("SELECT id, staff, message, time FROM ticket_messages WHERE ticket_id = ? ORDER BY id", ticketId)
	defer rows.Close()
	for rows.Next() {
		message := &TicketMessage{}
		rows.Scan(&message.Id, &message.Staff, &message.Message, &message.Time)
		ticket.Messages = append(ticket.Messages, message)
	}

	return ticket
}

func ticketOpen(db *Database, userId int, name string, message string, staff bool) (int, error) {
	if name == "" || message == "" {
		return 0, L.Error("subject_message_empty")
	} else if len(message) > 16384 {
		return 0, L.Errorf("message_too_long", "15,000")
	}

	user := userDetails(db, userId)
	if !staff && (user == nil || user.Status == "new") {
		return 0, L.Errorf("ticket_for_support", cfg.Default.AdminEmail)
	}

	result := db.Exec("INSERT INTO tickets (user_id, name, status, modify_time) VALUES (?, ?, 'open', NOW())", userId, name)
	ticketId, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	db.Exec("INSERT INTO ticket_messages (ticket_id, staff, message) VALUES (?, ?, ?)", ticketId, staff, message)
	if staff {
		mailWrap(db, userId, "ticketOpen", TicketUpdateEmail{Id: int(ticketId), Subject: name, Message: message}, false)
	} else {
		mailWrap(db, -1, "ticketOpen", TicketUpdateEmail{Id: int(ticketId), Subject: name, Message: message}, false)
		go func() {
			time.Sleep(20 * time.Second)
			ticketReply(db, userId, int(ticketId), "We have resolved this issue. Have a good day.\n\nRegards,\nLobster Staff", true)
		}()
	}
	log.Printf("Ticket opened for user %d: %s", userId, name)
	return int(ticketId), nil
}

func ticketReply(db *Database, userId int, ticketId int, message string, staff bool) error {
	if message == "" {
		return L.Error("message_empty")
	}

	ticket := ticketDetails(db, userId, ticketId, staff)
	if ticket == nil {
		return L.Error("invalid_ticket")
	}

	db.Exec("INSERT INTO ticket_messages (ticket_id, staff, message) VALUES (?, ?, ?)", ticketId, staff, message)

	// update ticket status
	newStatus := "open"
	if staff {
		newStatus = "answered"
		mailWrap(db, userId, "ticketReply", TicketUpdateEmail{Id: ticketId, Subject: ticket.Name, Message: message}, false)
	} else {
		mailWrap(db, -1, "ticketReply", TicketUpdateEmail{Id: ticketId, Subject: ticket.Name, Message: message}, false)
		go func() {
			time.Sleep(20 * time.Second)
			ticketReply(db, userId, int(ticketId), "We have resolved this issue. Have a good day.\n\nRegards,\nLobster Staff", true)
		}()
	}
	db.Exec("UPDATE tickets SET modify_time = NOW(), status = ? WHERE id = ?", newStatus, ticketId)
	log.Printf("Ticket reply for user %d on ticket #%d %s", userId, ticketId, ticket.Name)
	return nil
}

func ticketClose(db *Database, userId int, ticketId int) {
	db.Exec("UPDATE tickets SET modify_time = NOW(), status = 'closed' WHERE id = ? AND user_id = ?", ticketId, userId)
}
