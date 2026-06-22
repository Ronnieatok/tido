# TIDO — Turkana Inclusive Development Organization

Single-binary Go web server for the TIDO website. Server-rendered HTML
(`html/template`) with HTMX handling the contact form and newsletter
signup — no client-side validation JS, no CDN dependencies. Everything
(templates, CSS/JS, fonts, icons, logo, and the constitution/meeting
minutes PDFs) is embedded into the compiled binary via `go:embed`.

## Run locally

```bash
go run .
# -> TIDO server listening on http://localhost:8080
```

Custom port:

```bash
PORT=3000 go run .
```

## Build a deployable binary

```bash
go build -o tido-server .
./tido-server
```

The resulting `tido-server` binary is fully self-contained — copy it
anywhere with no `static/`, `templates/`, or `documents/` folders needed
alongside it; they're baked in.

Cross-compile for a Linux server from any machine:

```bash
GOOS=linux GOARCH=amd64 go build -o tido-server-linux .
```

## Project structure

```
main.go                        Routes, page data, validation, server
templates/index.html           Full page (header, hero, about, work,
                                leadership, get-involved, news, contact,
                                footer)
templates/partials/            HTMX-swapped fragments:
  contact_form.html              - the form itself (also used on first
                                    page load and to re-render with
                                    validation errors)
  contact_success.html           - swapped in after a valid submission
  newsletter.html                 - footer newsletter form + success/
                                    error states
static/css/style.css           Full design system (CSS custom
                                properties, components, responsive
                                breakpoints, HTMX request-state styles)
static/js/app.js               UI choreography only: mobile menu,
                                scroll-reveal, animated counters,
                                sticky header, back-to-top. No form
                                logic — that's server-side.
static/js/htmx.min.js          Vendored HTMX 2.0.4 (no CDN dependency)
static/vendor/fontawesome/     Vendored Font Awesome 6 (icons used
                                throughout)
static/fonts/                  Vendored Geist variable font
static/img/logo.png            TIDO logo
documents/                     Constitution & inaugural meeting minutes,
                                converted to PDF, served for download
```

## Routes

| Method | Path                    | Purpose                                   |
|--------|-------------------------|--------------------------------------------|
| GET    | `/`                     | Full page render                          |
| GET    | `/static/*`              | CSS, JS, fonts, icons, logo               |
| GET    | `/documents/*`            | Constitution / meeting minutes PDFs       |
| POST   | `/api/contact`            | HTMX: validates + returns a partial       |
| GET    | `/api/contact/reset`      | HTMX: returns a blank contact form        |
| POST   | `/api/newsletter`         | HTMX: validates email, returns a partial  |

## Editing content

Objectives, leadership roles, and news items are Go data (not hardcoded
HTML) — see `objectives()`, `leaders()`, and `newsItems()` near the top
of `main.go`. Edit those slices and rebuild.

Leadership names are placeholders (`"Chairperson Name"`, etc.), matching
the TIDO constitution's Article 6 positions — swap in real names once
elections are formalized.

## Known placeholders to fill in before going live

- M-Pesa Paybill number (`templates/index.html`, search `mpesa-details`)
- Email / phone / address (currently `info@tido.org`, `+254 700 000 000`,
  `Lodwar, Turkana County`)
- Social media links (currently `#`)
- Leadership names
- `/api/contact` and `/api/newsletter` handlers currently just log
  submissions (`log.Printf`) — wire them to real email delivery or a
  database before production use.
