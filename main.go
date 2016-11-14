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

type CalendarEvent struct {
	EventName   string
	Alert       int
	Description string
	StartDate   time.Time
	EndDate     time.Time
	UserId      int64
	Status      string
}

type User struct {
	Email          string
	FirstName      string
	LastName       string
	RegistrationID string
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

		fmt.Println(database)

		gocron.Every(1).Minute().Do(startPushNotification, database, c.String("cert_password"))
		<-gocron.Start()
		//startPushNotification()

		return nil
	}

	app.Run(os.Args)

}

func startPushNotification(database Database, certPassword string) {
	log.Println("Start the notification task")
	connectString := fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8&parseTime=true", database.User, database.Password, database.IP, database.Name)
	db, err := sql.Open("mysql", connectString)

	if err != nil {
		log.Fatal(err)
	}

	defer db.Close()

	result, err := db.Query("SELECT event_name, alert, COALESCE(description, '') as description, start_date, end_date, user_id, " +
		"status, email, last_name, first_name, registration_id FROM calendar_event c JOIN user u ON c.user_id = u.id " +
		"WHERE alert >= 36 AND start_date > now() AND start_date < now() + INTERVAL 1 MINUTE")

	if err != nil {
		log.Fatal(err)
	}

	for result.Next() {
		var calendarEvent CalendarEvent
		var calendarUser User

		result.Scan(&calendarEvent.EventName, &calendarEvent.Alert, &calendarEvent.Description, &calendarEvent.StartDate,
			&calendarEvent.EndDate, &calendarEvent.UserId, &calendarEvent.Status, &calendarUser.Email,
			&calendarUser.FirstName, &calendarUser.LastName, &calendarUser.RegistrationID)

		log.Println("------------------------------------")
		log.Printf("Event Name: %s, User Email: %s\n", calendarEvent.EventName, calendarUser.Email)
		log.Printf("%v \n", calendarEvent.StartDate)

		pushNotification(calendarEvent, calendarUser, certPassword)
	}
	log.Println("End the notification task")

}

func pushNotification(calendarEvent CalendarEvent, user User, certPassword string) {
	token := user.RegistrationID

	cert, err := certificate.Load("./cert/swing-push-product.p12", certPassword)
	panicError(err)

	client, err := push.NewClient(cert)
	panicError(err)

	certificate.TopicFromCert(cert)
	header := &push.Headers{}
	header.Topic = certificate.TopicFromCert(cert)

	service := push.NewService(client, push.Production)

	message := fmt.Sprintf("You have an event: %s", calendarEvent.EventName)

	p := payload.APS{
		Alert: payload.Alert{Body: message},
	}

	b, err := json.Marshal(p)
	panicError(err)

	id, err := service.Push(token, header, b)
	panicError(err)

	log.Printf("Pushed to %s\n", id)
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
		log.Println(err)
		return
	}
}

/**



 */
