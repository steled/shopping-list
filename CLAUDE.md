# CLAUDE.md — Arbeitsregeln für dieses Repository

## Pflichtschritte bei jeder Feature-Implementierung

Nach **jeder** Änderung an Go-Code, Templates oder statischen Assets sind folgende Schritte **automatisch und ohne gesonderte Aufforderung** durchzuführen:

### 1. Tests ausführen

```bash
go test ./...
```

Alle Tests müssen grün sein, bevor die Arbeit als abgeschlossen gilt. Schlägt ein Test fehl, ist zuerst die Ursache zu beheben — kein Überspringen.

### 2. Security-Analyse

Nach jeder Änderung, die neue Endpunkte, Authentifizierung, Cookie-Handling, Template-Rendering, Datenbankzugriffe oder externe Abhängigkeiten berührt, ist eine Security-Review mit `/security-review` durchzuführen.

Konkrete Auslöser (nicht abschließend):
- Neue HTTP-Handler oder Middleware
- Änderungen an `internal/auth/`
- Neue oder geänderte SQL-Queries
- Neue JavaScript-Dateien oder Template-Änderungen
- Neue Umgebungsvariablen mit sicherheitsrelevantem Inhalt
- Neue externe Abhängigkeiten (Go-Module, JS-Bibliotheken)

### 3. Dokumentation aktualisieren

`README.md` ist zu aktualisieren, wenn sich folgendes ändert:

| Änderung | README-Abschnitt |
|----------|-----------------|
| Neue/geänderte Umgebungsvariable | **Environment Variables** |
| Neuer/geänderter API-Endpunkt | **API Reference** |
| Neuer GitHub-Actions-Workflow | **CI/CD** |
| Neue Feature-Funktionalität | **Features** |
| Neue externe Abhängigkeit | **Tech Stack** |

---

## Projektkonventionen

### Commit-Format

Dieses Projekt verwendet [Conventional Commits](https://www.conventionalcommits.org/). Jeder Commit-Message-Prefix bestimmt das automatische Versioning:

| Prefix | Auswirkung |
|--------|-----------|
| `feat:` | Minor-Release |
| `fix:`, `perf:` | Patch-Release |
| `feat!:` / `BREAKING CHANGE` | Major-Release |
| `chore:`, `docs:`, `ci:`, `refactor:` | Kein Release |

### Code-Stil

- **Go**: Kein unnötiger Kommentar-Overhead. Kommentare nur wenn das *Warum* nicht offensichtlich ist.
- **SQL**: Ausschließlich parametrisierte Queries (`?`-Platzhalter) — niemals String-Konkatenation.
- **JavaScript**: Kein `eval()`, kein `innerHTML` mit nicht-escapten Benutzerdaten. Für Benutzerdaten `textContent` oder die `esc()`-Hilfsfunktion verwenden.
- **Templates**: Go's `html/template`-Paket wird verwendet — kein `template.HTML`-Cast ohne explizite Begründung.

### Sicherheitsregeln (nicht verhandelbar)

- Session-Cookies: immer `HttpOnly: true`, `SameSite: Lax`, `Secure: a.secureCookies || r.TLS != nil`
- Passwörter: ausschließlich bcrypt (`bcrypt.DefaultCost` oder höher)
- Session-Signatur: HMAC-SHA256, konstanter Zeitvergleich (`hmac.Equal`)
- `APP_SESSION_SECRET` muss mindestens 32 Zeichen haben (wird beim Start geprüft)
- CSP ist `default-src 'self'` — keine CDN-Whitelist ohne Bundling + explizite Begründung
- Neue externe JS-Bibliotheken: lokal in `static/` bundeln, Version in `static/<lib>.version` tracken

### Abhängigkeiten (JS-Bibliotheken)

Neue JavaScript-Bibliotheken werden **nicht** per CDN eingebunden, sondern:
1. In `static/<lib>.min.js` abgelegt
2. Version in `static/<lib>.version` festgehalten (für Renovate-Tracking)
3. Renovate `customManagers`-Eintrag in `renovate.json` ergänzen
4. GitHub Action nach dem Muster `.github/workflows/update-sortable.yml` erstellen

### Datenbankmigrationen

Schemaänderungen ausschließlich über die `migrate()`-Funktion in `internal/database/database.go`. Keine destruktiven Migrationen ohne explizite Bestätigung durch den Nutzer.

---

## Projektstruktur (Übersicht)

```
.
├── main.go                        # Einstiegspunkt, HTTP-Routing, Middleware
├── internal/
│   ├── auth/auth.go               # Session-Cookie, HMAC-Signatur, bcrypt
│   ├── database/database.go       # SQLite-Zugriff, alle SQL-Queries
│   └── handlers/handlers.go       # HTTP-Handler (Login, API, Templates)
├── templates/
│   ├── base.html                  # Basis-Layout (Header, Footer, Theme-Script-Ref)
│   ├── index.html                 # Einkaufsliste (referenziert /static/app.js)
│   └── login.html                 # Login-Formular
├── static/
│   ├── style.css                  # Styling
│   ├── app.js                     # Einkaufslisten-Logik (ausgelagert aus index.html)
│   ├── theme.js                   # Dark-Mode-Toggle (ausgelagert aus base.html)
│   ├── sortable.min.js            # SortableJS (lokal gebundelt)
│   └── sortable.version           # SortableJS-Version (Renovate-Ziel)
├── helm/shopping-list/            # Helm-Chart für Kubernetes
├── docker/Dockerfile              # Multi-Stage-Build
└── .github/workflows/             # CI/CD-Workflows
```

---

## Sicherheits-Checkliste für neue HTTP-Endpunkte

Bei jedem neuen Endpunkt prüfen:

- [ ] Authentifizierung: `h.RequireAuth(...)` gewrappt?
- [ ] Input-Validierung: Alle Felder validiert, Strings getrimmt?
- [ ] SQL: Parametrisierte Query, kein String-Formatting?
- [ ] Response: Korrekte HTTP-Statuscodes (201, 204, 400, 401, 404, 500)?
- [ ] Error-Handling: Kein Stack-Trace oder interne Details in der Response?
- [ ] Methode korrekt: GET für lesend, POST/PUT/PATCH/DELETE für schreibend?
