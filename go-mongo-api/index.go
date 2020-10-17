package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"

	"github.com/gorilla/mux"
)

//Meeting is..
type Meeting struct {
	ID               primitive.ObjectID `bson:"_id,omitempty"`
	Title            string             `bson:"title,omitempty"`
	Participants     []string           `bson:"participants,omitempty"`
	StartTime        string             `bson:"starttime,omitempty"`
	EndTime          string             `bson:"endtime,omitempty"`
	CreationTimestap string             `bson:"timestamp,omitempty"`
}

//People is...
type People struct {
	Name  string `bson:"name,omitempty"`
	Email string `bson:"email,omitempty"`
	RSVP  string `bson:"rsvp,omitempty"`
}

//Connection is..
type Connection struct {
	Meeting *mongo.Collection
	People  *mongo.Collection
}

//Page is..
type Page struct {
	Title             string
	Body              []byte
}

//Connection is..
func (connection Connection) CreateMeetingEndpoint(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("content-type", "application/json")
	var meeting Meeting
	if err := json.NewDecoder(request.Body).Decode(&meeting); err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{"message": "` + err.Error() + `"}`))
		return
	}
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	result, err := connection.Meeting.InsertOne(ctx, meeting)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{"message": "` + err.Error() + `"}`))
		return
	}
	json.NewEncoder(response).Encode(result)
}

//Connection is..
func (connection Connection) GetMeetingEndpoint(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("content-type", "application/json")
	var meeting []Meeting
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	cursor, err := connection.Meeting.Find(ctx, bson.M{})
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{"message": "` + err.Error() + `"}`))
		return
	}
	if err != cursor.All(ctx, &meeting); err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{"message": "` + err.Error() + `"}`))
		return
	}
	json.NewEncoder(response).Encode(meeting)
}

//Connection is..
func (connection Connection) UpdateMeetingEndpoint(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("content-type", "application/json")
	params := mux.Vars(request)
	id, _ := primitive.ObjectIDFromHex(params["id"])
	var meeting Meeting
	json.NewDecoder(request.Body).Decode(&meeting)
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	result, err := connection.Meeting.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.D{
			{"$set", meeting},
		},
	)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{"message": "` + err.Error() + `"}`))
		return
	}
	json.NewEncoder(response).Encode(result)
}

//Connection is..
func (connection Connection) DeleteMeetingEndpoint(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("content-type", "application/json")
	params := mux.Vars(request)
	id, _ := primitive.ObjectIDFromHex(params["id"])
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	result, err := connection.Meeting.DeleteOne(
		ctx,
		bson.M{"_id": id},
	)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{"message": "` + err.Error() + `"}`))
		return
	}
	json.NewEncoder(response).Encode(result)
}

func (p *Page) save() error {
	filename := p.Title + ".txt"
	return ioutil.WriteFile(filename, p.Body, 0600)
}

func loadPage(title string) (*Page, error) {
	filename := title + ".txt"
	body, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return &Page{Title: title, Body: body}, nil
}

func viewHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title)
	if err != nil {
		http.Redirect(w, r, "/edit/"+title, http.StatusFound)
		return
	}
	renderTemplate(w, "view", p)
}

func editHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title)
	if err != nil {
		p = &Page{Title: title}
	}
	renderTemplate(w, "edit", p)
}

func saveHandler(w http.ResponseWriter, r *http.Request, title string) {
	body := r.FormValue("body")
	p := &Page{Title: title, Body: []byte(body)}
	err := p.save()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/view/"+title, http.StatusFound)
}

var templates = template.Must(template.ParseFiles("edit.html", "view.html"))

func renderTemplate(w http.ResponseWriter, tmpl string, p *Page) {
	err := templates.ExecuteTemplate(w, tmpl+".html", p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

var validPath = regexp.MustCompile("^/(edit|save|view)/([a-zA-Z0-9]+)$")

func makeHandler(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := validPath.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.NotFound(w, r)
			return
		}
		fn(w, r, m[2])
	}
}

func main() {

	router := mux.NewRouter()
	router.HandleFunc("/meetings", connection.CreateMeetingEndpoint).Methods("POST")
	router.HandleFunc("/meeting/{id}", connection.UpdateMeetingEndpoint).Methods("PUT")
	router.HandleFunc("/meetings?start={starttime}&end={endtime}", connection.GetMeetingEndpoint).Methods("GET")
	router.HandleFunc("/meetings?participant={email}", connection.GetMeetingEndpoint).Methods("GET")
	http.ListenAndServe(":dbuserpassword", router)

	client, err := mongo.NewClient(options.Client().ApplyURI("mongodb+srv://dbuser:dbuserpassword@cluster0.w9ikw.mongodb.net/dbuser?retryWrites=true&w=majority"))
	if err != nil {
		log.Fatal(err)
	}
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	// client, err := mongo.Connect(ctx, options.Client().ApplyURI(os.Getenv("ATLAS_URI")))
	err = client.Connect(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Disconnect(ctx)
	if err = client.Ping(ctx, readpref.Primary()); err != nil {
		log.Fatal(err)
	}

	meetingDatabse := client.Database("Schedule")
	meetingCollection := meetingDatabse.Collection("meeting")
	participantCollection := meetingDatabse.Collection("people")

	peopleResult, err := participantCollection.InsertOne(ctx, bson.D{
		{Key: "name", Value: "Mr. singh"},
		{Key: "email", Value: "ms@gmail.com"},
		{Key: "RSVP", Value: "yes"},
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(peopleResult.InsertedID)

	document := bson.D{
		{"title", "Meeting 1"},
		{"starttime", "2: 00"},
		{"endtime", "5: 00"},
		{"participants", bson.A{"Mr. A", "Mr. B", "Mr. C"}},
	}
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Insert %v into meeting schedule collection\n", peopleResult.InsertedID)

	document := Meeting{
		Title:            "Meeting 1",
		Participants:     []string{"MR a", "MR b", "MR c"},
		StartTime:        "2:00",
		EndTime:          "4:00",
		CreationTimestap: "1:00",
	}

	meeting := Meeting{
		Title:            "Meeting 2",
		Participants:     []string{"MRr", "MR b", "MR q"},
		StartTime:        "1:00",
		EndTime:          "2:00",
		CreationTimestap: "11:00",
	}
	insertResult, err := meetingCollection.InsertOne(ctx, meeting)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(insertResult.InsertedID)

	insertResult, err := meetingCollection.InsertMany(ctx, []interface{}{
		bson.D{
			{"title", "Meeting 4"},
			{"participants", bson.A{"Mr. i", "Mr. o", "Mr. u"}},
			{"starttime", "5: 00"},
			{"endtime", "8: 00"},
			{"timestamp", "4:40"},
		},
		bson.D{
			{"title", "Meeting 5"},
			{"participants", bson.A{"Mr. i", "Mr. o", "Mr. u"}},
			{"starttime", "8: 00"},
			{"endtime", "9: 00"},
			{"timestamp", "4:40"},
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Insertes %v meetings into meeting collection\n", len(insertResult.InsertedIds)

	filterCursor, err := meetingCollection.Find(ctx, bson.M{"starttime": "2:00"})
	if err != nil {
		log.Fatal(err)
	}
	defer filterCursor.Close(ctx)
	for filterCursor.Next(ctx) {
		var meeting Meeting
		if err = filterCursor.Decode(&meeting); err != nil {
			log.Fatal(err)
		}
		fmt.Println(meeting)
	}

	result, err = meetingCollection.UpdateMany(
		ctx,
		bson.M{"title": "Meeting 11"},
		bson.D{
			{"$set", bson.D{{"starttime", "8:00"}}},
		},
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Updated %v meetings\n", result.ModifiedCount)

	result, err = meetingCollection.DeleteMany(ctx, bson.M{"endtime": "8:00"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("DeletedMany removed %v meetings\n", result.DeletedCount)

	http.HandleFunc("/view/", makeHandler(viewHandler))
	http.HandleFunc("/edit/", makeHandler(editHandler))
	http.HandleFunc("/save/", makeHandler(saveHandler))

	log.Fatal(http.ListenAndServe(":8081", nil))
}
