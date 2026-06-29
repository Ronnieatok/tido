// Command tido-go serves the Turkana Inclusive Development Organization
// website. It is a self-contained binary: templates, CSS/JS, the logo,
// and the constitution/meeting-minutes PDFs are all embedded via go:embed,
// so the compiled binary can be deployed and run with no extra files.
//
// Interactivity (contact form, newsletter signup) is handled with HTMX:
// the browser POSTs a normal form, the server validates it, and returns
// an HTML fragment that HTMX swaps into the page. There is no client-side
// validation JS — all of it lives here, server-side, in Go.
package main

import (
	"embed"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

//go:embed templates/*.html templates/partials/*.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

//go:embed documents
var documentsFS embed.FS

var templates *template.Template

// ---------------------------------------------------------------------
// Page data
// ---------------------------------------------------------------------

// Objective is one of the 10 constitutional focus areas shown in "Our Work".
type Objective struct {
	Index  string // "01".."10" — used as the card's decorative number
	Icon   string // Font Awesome solid icon class, e.g. "fa-graduation-cap"
	IconBg string // icon badge background variant: bg-teal | bg-orange | bg-navy | bg-ochre
	Title  string
	Desc   string
}

// Leader is one of the 9 executive committee positions defined in
// Article 6 of the TIDO constitution. Names are placeholders pending
// formal appointment (the source documents leave these blank too).
type Leader struct {
	Initials string
	IconBg   string
	Name     string
	Role     string
	Desc     string
}

// NewsItem is a card in the News & Updates section.
type NewsItem struct {
	Date       string
	Title      string
	Excerpt    string
	Icon       string
	ThumbClass string // t1 | t2 | t3 — gradient variant
}

// ContactFormState drives the contact-form partial: pre-filled values,
// any field-level error messages, and an optional banner error.
type ContactFormState struct {
	Values      map[string]string
	Errors      map[string]string
	GlobalError string
}

func emptyContactForm() ContactFormState {
	return ContactFormState{
		Values: map[string]string{"name": "", "email": "", "phone": "", "subject": "", "message": ""},
		Errors: map[string]string{},
	}
}

// PageData is passed to templates/index.html.
type PageData struct {
	Objectives  []Objective
	Leaders     []Leader
	News        []NewsItem
	ContactForm ContactFormState
	Year        int
}

func objectives() []Objective {
	return []Objective{
		{"01", "fa-graduation-cap", "bg-teal", "Education & Skills Development", "Promoting education and skills development among youth and vulnerable groups."},
		{"02", "fa-wheelchair", "bg-orange", "Disability Empowerment", "Empowering persons with disabilities through advocacy and inclusion programs."},
		{"03", "fa-hand-holding-dollar", "bg-navy", "Economic Empowerment", "Supporting economic empowerment and entrepreneurship initiatives."},
		{"04", "fa-child-reaching", "bg-ochre", "Child & Youth Protection", "Promoting child protection and youth development across communities."},
		{"05", "fa-scale-balanced", "bg-teal", "Human Rights & Justice", "Advocating for human rights, equality, and social justice for all."},
		{"06", "fa-heart-pulse", "bg-orange", "Community Health", "Supporting community health and well-being initiatives."},
		{"07", "fa-seedling", "bg-navy", "Environmental Resilience", "Promoting environmental conservation and climate resilience."},
		{"08", "fa-handshake-angle", "bg-ochre", "Peacebuilding & Cohesion", "Strengthening peacebuilding and community cohesion in the region."},
		{"09", "fa-laptop-code", "bg-teal", "ICT & Digital Innovation", "Promoting ICT and digital innovation for inclusive development."},
		{"10", "fa-people-group", "bg-orange", "Strategic Collaboration", "Collaborating with government, NGOs, donors, and community stakeholders."},
	}
}

func leaders() []Leader {
	return []Leader{
		{"KL", "bg-navy", "Kelvin Loibach", "Chairperson", "Provides overall leadership and direction, chairs meetings, and represents TIDO externally."},
		{"AM", "bg-teal", "Akai Mary", "Vice Chairperson", "Supports the Chairperson and steps in to lead in their absence."},
		{"EJ", "bg-orange", "Edapal James", "Secretary", "Keeps official records and minutes, and handles organizational communication."},
		{"MA", "bg-ochre", "Mariah Akiru", "Assistant Secretary", "Supports the Secretary in maintaining records and membership documentation."},
		{"RA", "bg-navy", "Ronnie Atok", "Treasurer", "Manages organizational finances and prepares financial reports for members."},
		{"NE", "bg-teal", "Nanok Edapal", "Organizing Secretary", "Coordinates events, meetings, and on-the-ground community mobilization."},
		{"CE", "bg-orange", "Chris Ekiru", "Coordinator, Persons with Disabilities", "Champions disability inclusion and advocacy within all TIDO programs."},
		{"MI", "bg-ochre", "Mercy Idome", "Youth Representative", "Represents the voice and interests of young people across the organization."},
		{"AA", "bg-navy", "Avrila Adome", "Women Representative", "Champions women's empowerment and representation in decision-making."},
	}
}

func newsItems() []NewsItem {
	return []NewsItem{
		{"Inaugural Meeting", "TIDO holds founding meeting and adopts constitution", "Members convened to formally adopt the TIDO name, vision, mission, and constitution, and elected interim officials to steer the organization forward.", "fa-people-group", "t1"},
		{"Coming Soon", "Disability inclusion outreach planned across the county", "TIDO is preparing community awareness activities focused on disability inclusion, accessibility, and advocacy for persons with disabilities.", "fa-wheelchair", "t2"},
		{"Coming Soon", "Registration process underway with government authorities", "The Secretary and Chairperson are coordinating TIDO's official registration to formalize the organization's legal standing.", "fa-seedling", "t3"},
	}
}

// ---------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------

var emailRe = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
var phoneRe = regexp.MustCompile(`^[0-9+()\-\s]{7,}$`)

// validateContact mirrors the original client-side rules, enforced here
// server-side: name/subject >= 2 chars, valid email, optional-but-valid
// phone, message >= 10 chars.
func validateContact(name, email, phone, subject, message string) map[string]string {
	errs := map[string]string{}

	if len(strings.TrimSpace(name)) < 2 {
		errs["name"] = "Please enter your name."
	}
	if !emailRe.MatchString(strings.TrimSpace(email)) {
		errs["email"] = "Please enter a valid email."
	}
	if p := strings.TrimSpace(phone); p != "" && !phoneRe.MatchString(p) {
		errs["phone"] = "Please enter a valid phone number."
	}
	if len(strings.TrimSpace(subject)) < 2 {
		errs["subject"] = "Please enter a subject."
	}
	if len(strings.TrimSpace(message)) < 10 {
		errs["message"] = "Please enter a message (min. 10 characters)."
	}
	return errs
}

// ---------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data := PageData{
		Objectives:  objectives(),
		Leaders:     leaders(),
		News:        newsItems(),
		ContactForm: emptyContactForm(),
		Year:        time.Now().Year(),
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, "index.html", data); err != nil {
		log.Printf("render index: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

// handleContactSubmit validates the posted contact form. On success it
// renders the "contact-success" partial; on failure it re-renders
// "contact-form" with field errors and the values the visitor typed,
// so nothing is lost on a mistake.
func handleContactSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	email := r.FormValue("email")
	phone := r.FormValue("phone")
	subject := r.FormValue("subject")
	message := r.FormValue("message")

	errs := validateContact(name, email, phone, subject, message)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if len(errs) > 0 {
		state := ContactFormState{
			Values: map[string]string{
				"name": name, "email": email, "phone": phone,
				"subject": subject, "message": message,
			},
			Errors:      errs,
			GlobalError: "Please correct the highlighted fields.",
		}
		if err := templates.ExecuteTemplate(w, "contact-form", state); err != nil {
			log.Printf("render contact-form: %v", err)
		}
		return
	}

	// In production this is where you'd send an email / persist to a
	// database / push to a queue. Logging stands in for that here.
	log.Printf("contact message received: name=%q email=%q subject=%q", name, email, subject)

	first := strings.TrimSpace(name)
	if i := strings.IndexByte(first, ' '); i > 0 {
		first = first[:i]
	}
	if err := templates.ExecuteTemplate(w, "contact-success", struct{ FirstName string }{first}); err != nil {
		log.Printf("render contact-success: %v", err)
	}
}

// handleContactReset returns a blank contact-form partial — used by the
// "Send Another Message" button after a successful submission.
func handleContactReset(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, "contact-form", emptyContactForm()); err != nil {
		log.Printf("render contact-form (reset): %v", err)
	}
}

// handleNewsletterSubmit validates an email and renders a success or
// error partial for the footer newsletter signup.
func handleNewsletterSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	email := strings.TrimSpace(r.FormValue("email"))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if !emailRe.MatchString(email) {
		if err := templates.ExecuteTemplate(w, "newsletter-error", email); err != nil {
			log.Printf("render newsletter-error: %v", err)
		}
		return
	}

	log.Printf("newsletter signup: email=%q", email)
	if err := templates.ExecuteTemplate(w, "newsletter-success", nil); err != nil {
		log.Printf("render newsletter-success: %v", err)
	}
}

// ---------------------------------------------------------------------
// main
// ---------------------------------------------------------------------

func main() {
	var err error
	templates, err = template.ParseFS(templateFS, "templates/*.html", "templates/partials/*.html")
	if err != nil {
		log.Fatalf("parse templates: %v", err)
	}

	staticSub, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Fatalf("static sub fs: %v", err)
	}
	documentsSub, err := fs.Sub(documentsFS, "documents")
	if err != nil {
		log.Fatalf("documents sub fs: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndex)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))
	mux.Handle("/documents/", http.StripPrefix("/documents/", http.FileServer(http.FS(documentsSub))))
	mux.HandleFunc("/api/contact", handleContactSubmit)
	mux.HandleFunc("/api/contact/reset", handleContactReset)
	mux.HandleFunc("/api/newsletter", handleNewsletterSubmit)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      logRequests(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("TIDO server listening on http://localhost:%s", port)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

// logRequests is a tiny access-log middleware.
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
