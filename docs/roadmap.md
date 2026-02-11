# Roadmap (MVP)

## 1) Kurzbeschreibung des Produkts
Die App hilft Nutzer:innen dabei, spontane Käufe bewusster zu entscheiden, indem gewünschte Produkte zuerst auf eine Warteliste gesetzt werden. Für jedes Item wird eine Wartezeit festgelegt; erst nach Ablauf ist ein Kauf ausdrücklich „erlaubt“. Zusätzlich zeigt die App einen Reality-Check über benötigte Arbeitsstunden auf Basis eines hinterlegten Profil-Stundenlohns. Das MVP ist lokal, self-hosted und auf einen ultraschnellen mobilen Add-Flow ausgelegt.

## 2) Glossar der wichtigsten Begriffe
- **Item**: Ein potenzieller Kauf (Wunschobjekt) mit den Feldern Titel, Preis, Link, Notiz und Kategorie/Tags.
- **Wartezeit**: Zeitraum zwischen Erstellen des Items und dem Zeitpunkt, ab dem ein Kauf erlaubt ist (z. B. 24h, 7 Tage, 30 Tage, Custom).
- **Status**: Lebenszyklus eines Items: `Wartet` → `Kauf erlaubt` → `Gekauft` oder `Nicht gekauft`.
- **Kategorie/Tags**: Freie Klassifizierung eines Items zur späteren Auswertung (z. B. „Tech“, „Kleidung“).
- **Profil-Stundenlohn**: Lokal gespeicherter Netto-Stundenwert (Zahl) des einzigen MVP-Nutzerprofils; Basis für den Stunden-Reality-Check.
- **ntfy**: Konfigurierbarer Benachrichtigungs-Endpunkt mit Topic; im MVP wird beim Statuswechsel zu `Kauf erlaubt` eine Erinnerung gesendet.
- **„gespart“**: Summe der Preise aller Items mit Status `Nicht gekauft` (MVP-Definition).

### Produktentscheidungen (MVP)
- **Minimal-Usermanagement**: Genau ein lokales Nutzerprofil, kein Login.
- **Speicherung**: Lokal (z. B. SQLite), Betrieb self-hosted.
- **Mobile-First UX**: Add-Flow auf dem Handy mit sinnvollen Defaults und minimalen Pflichtfeldern, Ziel <10 Sekunden.
- **ntfy-Umfang**: Ein Endpunkt + Topic global konfigurierbar; Versand beim Übergang in `Kauf erlaubt`.

## 3) MVP – User Stories (priorisiert, klein geschnitten)

### (done) MVP-001 — Item minimal anlegen (mobile-first Add-Flow) 
**User Story**  
Als Käufer:in möchte ich ein Item mit minimalen Eingaben sehr schnell erfassen, damit ich Impulskäufe sofort in die Warteliste umleiten kann.

**Akzeptanzkriterien (Given/When/Then)**
1. **Given** ich bin auf der Add-Ansicht am Handy, **when** ich nur einen Titel eingebe und speichere, **then** wird das Item erfolgreich mit sinnvollen Defaults angelegt.
2. **Given** ich lege ein Item an, **when** ich keinen Preis, Link, Notiz oder Tags setze, **then** sind diese Felder optional und blockieren das Speichern nicht.
3. **Given** ein Item wurde gespeichert, **when** die Liste aktualisiert wird, **then** erscheint das neue Item sofort mit Status `Wartet`.
4. **Given** ich sende das Formular mit leerem Titel, **when** ich speichern möchte, **then** erhalte ich eine klare Validierung und kein Item wird angelegt.
5. **Given** ich nutze den Add-Flow auf Mobile, **when** ich ein Standard-Item erfasse, **then** ist der Flow auf wenige Interaktionen optimiert (keine unnötigen Pflichtschritte).

**Out of Scope**
- Bulk-Import, OCR, Browser-Extensions.
- Mehrbenutzerfähigkeit oder Login.

**Notizen/Offene Fragen**
- Konkrete Performance-Messung „<10 Sekunden“: manuell mit E2E-Timer oder Telemetrie?

---

### (done) MVP-002 — Wartezeit-Auswahl inkl. Custom
**User Story**  
Als Käufer:in möchte ich beim Anlegen eine Wartezeit auswählen, damit das Item erst nach einer bewussten Pause kaufbar ist.

**Akzeptanzkriterien (Given/When/Then)**
1. **Given** ich öffne das Add-Formular, **when** ich die Wartezeit auswähle, **then** stehen Presets `24h`, `7 Tage`, `30 Tage` und `Custom` zur Verfügung.
2. **Given** ich nutze ein Preset, **when** ich speichere, **then** wird der Zeitpunkt `kauf_erlaubt_ab` korrekt aus Erstellzeit + Preset berechnet.
3. **Given** ich wähle `Custom`, **when** ich eine gültige Dauer eingebe und speichere, **then** wird diese Dauer persistiert und korrekt berechnet.
4. **Given** ich gebe bei `Custom` eine ungültige Dauer ein (z. B. 0 oder negativ), **when** ich speichern möchte, **then** erhalte ich eine Validierungsmeldung.

**Out of Scope**
- Komplexe wiederkehrende Erinnerungspläne.
- Kalenderintegration.

**Notizen/Offene Fragen**
- Einheit für `Custom` finalisieren (Stunden/Tage oder ISO-Dauer).

---

### (done) MVP-003 — Status-Automatik und manuelle Entscheidung
**User Story**  
Als Käufer:in möchte ich einen klaren Statusverlauf sehen und final entscheiden können, damit mein Kaufverhalten nachvollziehbar bleibt.

**Akzeptanzkriterien (Given/When/Then)**
1. **Given** ein Item ist neu, **when** es erstellt wird, **then** startet es immer im Status `Wartet`.
2. **Given** die Wartezeit ist abgelaufen, **when** ein Status-Check läuft (bei Seitenaufruf/Job), **then** wechselt das Item automatisch in `Kauf erlaubt`.
3. **Given** ein Item steht auf `Kauf erlaubt`, **when** ich `Gekauft` auswähle, **then** wird der Status persistiert und nicht mehr automatisch geändert.
4. **Given** ein Item steht auf `Kauf erlaubt`, **when** ich `Nicht gekauft` auswähle, **then** wird der Status persistiert und für Spar-Auswertung berücksichtigt.
5. **Given** ein Item ist bereits `Gekauft` oder `Nicht gekauft`, **when** Zeit vergeht, **then** erfolgt kein Rücksprung auf frühere Stati.

**Out of Scope**
- Teilkäufe, Rückgaben, mehrere Kaufzeitpunkte.
- Historisierung aller Statuswechsel mit Audit-Log.

**Notizen/Offene Fragen**
- Klären, ob manueller Wechsel auf `Nicht gekauft` auch aus `Wartet` erlaubt sein soll (derzeit nur ab `Kauf erlaubt`).

---

### (done) MVP-004 — Profil mit Stundenlohn (Single User, lokal)
**User Story**  
Als Nutzer:in möchte ich meinen Netto-Stundenwert einmalig konfigurieren, damit die App mir Arbeitsstunden als Reality-Check anzeigen kann.

**Akzeptanzkriterien (Given/When/Then)**
1. **Given** ich öffne den Profilbereich, **when** noch kein Profil existiert, **then** kann ich genau ein lokales Profil mit Stundenlohn anlegen.
2. **Given** ich speichere einen gültigen Stundenlohn (>0), **when** ich später die Seite neu lade, **then** bleibt der Wert persistent gespeichert.
3. **Given** ich gebe einen ungültigen Wert (leer, 0, negativ, kein Zahlformat) ein, **when** ich speichern möchte, **then** wird eine Validierung angezeigt.
4. **Given** es existiert bereits ein Profil, **when** ich den Stundenlohn ändere, **then** wird derselbe Datensatz aktualisiert statt ein zweites Profil zu erzeugen.

**Out of Scope**
- Login, Registrierung, Mehrbenutzerprofile.
- Währungen, Steuerlogik, Gehaltshistorie.

**Notizen/Offene Fragen**
- Rundungsregel für Anzeige (z. B. 1 Nachkommastelle) im UI vereinbaren.

---

### (done) MVP-005 — Reality-Check: Arbeitsstunden pro Item
**User Story**  
Als Käufer:in möchte ich pro Item sehen, wie viele Arbeitsstunden der Preis entspricht, damit ich Kaufentscheidungen realistischer treffe.

**Akzeptanzkriterien (Given/When/Then)**
1. **Given** ein Item hat einen Preis und ein Profil-Stundenlohn ist gesetzt, **when** ich das Item sehe, **then** wird `Preis / Stundenlohn` als Arbeitsstunden angezeigt.
2. **Given** der Stundenlohn wird geändert, **when** ich Items erneut aufrufe, **then** wird die Stundenanzeige mit dem neuen Wert berechnet.
3. **Given** ein Item hat keinen Preis oder kein gültiger Stundenlohn ist vorhanden, **when** ich das Item sehe, **then** wird keine fehlerhafte Zahl angezeigt, sondern ein neutraler Hinweis.
4. **Given** Berechnungen liefern Dezimalwerte, **when** die UI rendert, **then** erfolgt konsistente Rundung gemäß definierter Regel.

**Out of Scope**
- Kaufkraftanpassung, Inflation, Opportunitätskosten.
- Vergleich über mehrere Personen.

**Notizen/Offene Fragen**
- Soll der Hinweistext bei fehlenden Daten klickbar zum Profil führen?

---

### (done) MVP-006 — Basis-Auswertung (nicht gekauft, gespart, Top-Kategorien)
**User Story**  
Als Nutzer:in möchte ich eine einfache Auswertung sehen, damit ich den Effekt meiner Kaufpausen erkenne.

**Akzeptanzkriterien (Given/When/Then)**
1. **Given** es gibt Items mit unterschiedlichen Stati, **when** ich das Dashboard öffne, **then** sehe ich die Anzahl `Nicht gekauft`.
2. **Given** es gibt Items mit Status `Nicht gekauft` und Preis, **when** die Kennzahl berechnet wird, **then** entspricht `gespart` der Summe dieser Preise.
3. **Given** Items haben Kategorie/Tags, **when** ich die Auswertung sehe, **then** werden Top-Kategorien nach Anzahl der Items angezeigt.
4. **Given** keine Daten sind vorhanden, **when** ich das Dashboard öffne, **then** sehe ich einen leeren, verständlichen Zero-State statt Fehler.
5. **Given** ich ändere einen relevanten Status, **when** ich aktualisiere, **then** spiegeln die Kennzahlen die neuen Werte korrekt wider.

**Out of Scope**
- Zeitreihencharts, Export, BI-Integration.
- Monetäre Auswertung nach Währungen.

**Notizen/Offene Fragen**
- Für Top-Kategorien: Zählbasis final festlegen (alle Items vs. nur `Nicht gekauft`).

---

### (done) MVP-007 — ntfy-Erinnerung bei „Kauf erlaubt“
**User Story**  
Als Nutzer:in möchte ich bei Ablauf der Wartezeit eine Benachrichtigung erhalten, damit ich bewusst über den Kauf entscheiden kann.

**Akzeptanzkriterien (Given/When/Then)**
1. **Given** in den Einstellungen sind ntfy-Endpunkt und Topic konfiguriert, **when** ein Item von `Wartet` auf `Kauf erlaubt` wechselt, **then** sendet die App genau eine ntfy-Nachricht pro Item-Übergang.
2. **Given** ein Item ist bereits `Kauf erlaubt`, **when** weitere Status-Checks laufen, **then** wird keine doppelte Erinnerung gesendet.
3. **Given** ntfy ist nicht konfiguriert, **when** ein Item `Kauf erlaubt` wird, **then** bleibt der Statuswechsel erhalten und die App loggt einen nachvollziehbaren Hinweis statt zu crashen.
4. **Given** der ntfy-Request schlägt fehl (z. B. 5xx/Timeout), **when** der Versand versucht wird, **then** wird der Fehler protokolliert und der Nutzerfluss bleibt funktionsfähig.

**Out of Scope**
- Push über weitere Kanäle (E-Mail, SMS, Pushover).
- Mehrere Topics pro Kategorie.

**Notizen/Offene Fragen**
- Retry-Strategie im MVP: sofortiger einmaliger Versuch vs. einfacher Backoff.

---

### (done) MVP-008 — Lokale Persistenz & Self-hosted Betriebsfähigkeit
**User Story**  
Als Betreiber:in möchte ich die App lokal und self-hosted mit persistenter Datenhaltung betreiben, damit meine Daten ohne Cloud-Abhängigkeit verfügbar bleiben.

**Akzeptanzkriterien (Given/When/Then)**
1. **Given** ich starte die App per Docker Compose, **when** der Container hochfährt, **then** ist die Anwendung lokal erreichbar.
2. **Given** ich lege Profil und Items an, **when** ich den Container neu starte, **then** bleiben die Daten in der lokalen DB erhalten.
3. **Given** die Datenbankdatei fehlt beim Start, **when** die App initialisiert, **then** wird das benötigte Schema automatisch erstellt.
4. **Given** ein Laufzeitfehler in der DB-Verbindung tritt auf, **when** die App startet/arbeitet, **then** erhalte ich einen klaren Fehlerhinweis im Log.

**Out of Scope**
- Cloud-Sync, Multi-Node-Betrieb, HA-Setups.
- Mandantenfähigkeit.

**Notizen/Offene Fragen**
- Speicherort der SQLite-Datei in Docker-Setup verbindlich dokumentieren.

## 4) Definition of Done (DoD) für jede Story
- App lässt sich lokal starten (Docker empfohlen).
- Unit Tests vorhanden und grün.
- E2E Tests mit Playwright (Happy Path) vorhanden und grün.
- Exploratory UI Checks (Playwright „explore“ Suite: Navigation/Form inputs/Console errors/4xx-5xx) vorhanden und grün.
- README enthält „Run“ und „Test“ Schritte.

## 5) R1 candidates
- Bearbeiten/Löschen von Items.
- Filter, Sortierung und Suche in der Item-Liste.
- Erweiterte Statistiken (Zeitreihen, Trends pro Monat).
- Mehrere ntfy-Profile/Topics je Kategorie.
- Import aus Wunschlisten/Bookmarks.
- Optionales Login/Mehrbenutzerfähigkeit.
