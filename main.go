package main

import (
	"database/sql"
	"os"
	"strconv"

	"encoding/json"
	"fmt"
	"log"
	"net/smtp"
	"time"

	"github.com/RobotsAndPencils/buford/certificate"
	"github.com/RobotsAndPencils/buford/payload"
	"github.com/RobotsAndPencils/buford/push"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jasonlvhit/gocron"
	"github.com/urfave/cli"
)

type NotificationData struct {
	Event CalendarEvent
	User  User
}

type CalendarEvent struct {
	ID          int64
	EventName   string
	Alert       int
	Description string
	PushDate    time.Time
	Weekday     string
	Repeat      string
	UserId      int64
	Status      string
}

type User struct {
	Email          string
	FirstName      string
	LastName       string
	RegistrationID string
	Lang           string
}

type Database struct {
	Name     string
	User     string
	Password string
	IP       string
}

func main() {

	app := cli.NewApp()
	app.Name = "Swing-Push-Worker"

	app.Flags = []cli.Flag{
		cli.StringFlag{
			EnvVar: "DATABASE_USER",
			Name:   "database_user",
			Usage:  "Database user name",
			Value:  "",
		},
		cli.StringFlag{
			EnvVar: "DATABASE_PASSWORD",
			Name:   "database_password",
			Usage:  "Database password",
			Value:  "",
		},
		cli.StringFlag{
			EnvVar: "DATABASE_IP",
			Name:   "database_IP",
			Usage:  "Database IP address with port number",
			Value:  "",
		},
		cli.StringFlag{
			EnvVar: "DATABASE_NAME",
			Name:   "database_name",
			Usage:  "Database name",
			Value:  "childre_qu",
		},
		cli.StringFlag{
			EnvVar: "CERT_PASSWORD",
			Name:   "cert_password",
			Usage:  "Push cert password",
			Value:  "",
		},
	}

	app.Action = func(c *cli.Context) error {
		database := Database{
			Name:     c.String("database_name"),
			User:     c.String("database_user"),
			Password: c.String("database_password"),
			IP:       c.String("database_IP"),
		}

		gocron.Every(1).Minute().Do(startPushNotification, database, c.String("cert_password"))
		<-gocron.Start()
		//startPushNotification()

		return nil
	}

	app.Run(os.Args)

}

func connectToDatabase(database Database) *sql.DB {
	connectString := fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8&parseTime=true", database.User, database.Password, database.IP, database.Name)
	db, err := sql.Open("mysql", connectString)

	if err != nil {
		log.Fatal(err)
	}

	return db
}

func startPushNotification(database Database, certPassword string) {
	//log.Println("Check the notification task")

	db := connectToDatabase(database)
	defer db.Close()

	notificationEvent, err := db.Query("SELECT c.id, i.text, alert, COALESCE(description, '') as description, push_time_utc, user_id, " +
		"status, email, last_name, first_name, registration_id, language FROM event c JOIN user u ON c.user_id = u.id JOIN i18n_alert i ON i.alert_id = c.alert AND i.lang = u.language " +
		"WHERE alert >= 36 AND status != 'NOTIFICATION_SENT' AND (`repeat` = '' or `repeat` is null) AND registration_id != '' AND registration_id is not null AND push_time_utc >= now() AND push_time_utc <= now() + INTERVAL 1.1 MINUTE")

	if err != nil {
		log.Fatal(err)
	}

	var notificationDatas []NotificationData

	for notificationEvent.Next() {
		var calendarEvent CalendarEvent
		var calendarUser User

		notificationEvent.Scan(&calendarEvent.ID, &calendarEvent.EventName, &calendarEvent.Alert, &calendarEvent.Description,
			&calendarEvent.PushDate, &calendarEvent.UserId, &calendarEvent.Status, &calendarUser.Email,
			&calendarUser.FirstName, &calendarUser.LastName, &calendarUser.RegistrationID, &calendarUser.Lang)

		var notificationData NotificationData
		notificationData.Event = calendarEvent
		notificationData.User = calendarUser

		notificationDatas = append(notificationDatas, notificationData)

	}

	repeatEvent, err := db.Query("SELECT c.id, i.text, alert, COALESCE(description, '') as description, DAYNAME(push_time_utc) as weekday, push_time_utc," +
		" `repeat`, user_id, status, email, last_name, first_name, registration_id, language FROM event c JOIN user u ON c.user_id = u.id JOIN i18n_alert i" +
		" ON i.alert_id = c.alert AND i.lang = u.language WHERE alert >= 36 AND (`repeat` != '' AND `repeat` is not null) AND registration_id != '' AND registration_id is not null AND " +
		"hour(push_time_utc) = hour(now()) AND minute(push_time_utc) = minute(now())")

	if err != nil {
		log.Fatal(err)
	}

	for repeatEvent.Next() {
		var calendarEvent CalendarEvent
		var calendarUser User

		repeatEvent.Scan(&calendarEvent.ID, &calendarEvent.EventName, &calendarEvent.Alert, &calendarEvent.Description, &calendarEvent.Weekday,
			&calendarEvent.PushDate, &calendarEvent.Repeat, &calendarEvent.UserId, &calendarEvent.Status, &calendarUser.Email,
			&calendarUser.FirstName, &calendarUser.LastName, &calendarUser.RegistrationID, &calendarUser.Lang)

		var notificationData NotificationData
		notificationData.Event = calendarEvent
		notificationData.User = calendarUser

		notificationDatas = append(notificationDatas, notificationData)

	}

	if len(notificationDatas) > 0 {
		pushNotification(notificationDatas, certPassword)
	}

}

func pushNotification(notificationDatas []NotificationData, certPassword string) {
	cert, err := certificate.Load("./cert/com_kd_swing.p12", certPassword)
	panicError(err)

	client, err := push.NewClient(cert)
	panicError(err)

	certificate.TopicFromCert(cert)
	header := &push.Headers{}
	header.Topic = certificate.TopicFromCert(cert)

	service := push.NewService(client, push.Production)

	for _, notificationData := range notificationDatas {
		log.Println("------------------------------------")
		log.Printf("Process: %#v \n", notificationData)

		if notificationData.Event.Repeat != "" {
			if !SendRepeatNotification(notificationData.Event) {
				log.Println("Weekly event, but it's not today")
				log.Println("------------------------------------")
				continue
			}
		}

		message := fmt.Sprintf("You have an event: %s", notificationData.Event.EventName)

		p := payload.APS{
			Alert: payload.Alert{Body: message},
		}

		b, err := json.Marshal(p)
		panicError(err)

		id, err := service.Push(notificationData.User.RegistrationID, header, b)
		panicError(err)

		log.Printf("Pushed to %s\n", id)
		log.Println("------------------------------------")
	}

}

func SendRepeatNotification(event CalendarEvent) bool {
	if event.Repeat == "WEEKLY" && time.Now().UTC().Weekday().String() == event.Weekday {
		return true
	} else if event.Repeat == "DAILY" {
		return true
	}

	return false
}

type EmailUser struct {
	Username    string
	Password    string
	EmailServer string
	Port        int
}

func sendBugMail(err string) {
	emailUser := &EmailUser{"", "", "smtp.gmail.com", 587}

	sendMail(emailUser, "bug@kidsdynamic.com", err)
}

func sendMail(emailUser *EmailUser, toEmail, message string) {

	auth := smtp.PlainAuth(
		"",
		emailUser.Username,
		emailUser.Password,
		emailUser.EmailServer,
	)
	// Connect to the server, authenticate, set the sender and recipient,
	// and send the email all in one step.
	err := smtp.SendMail(
		emailUser.EmailServer+":"+strconv.Itoa(emailUser.Port),
		auth,
		emailUser.Username,
		[]string{"jack08300@gmail.com"},
		[]byte("This is the debug email."),
	)
	if err != nil {
		log.Fatal(err)
	}
}

func panicError(err error) {
	if err != nil {
		//sendBugMail(err.Error())
		fmt.Println(err)
		return
	}
}
