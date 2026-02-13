# Definition of Done (DoD)

Zentrale, projektweite DoD auf Basis der MVP-DoD.
Diese Definition wird von den Roadmap-Dokumenten referenziert, damit sie nicht mehrfach gepflegt werden muss.

## Standard-DoD für jede Story
- App lässt sich lokal starten (Docker empfohlen).
- Jedes Acceptance Criterion (AC) ist durch automatisierte Tests abgedeckt (mindestens einer pro AC): Unit-, E2E- oder Smoke-Tests.
- Alle vorhandenen automatisierten Tests sind grün.
- Es wurde zusätzlich ein neuer „monkeyish“ Test ergänzt (zufallsnahe bzw. robustheitsorientierte Interaktionen), automatisiert und grün.
- Neue Funktionen erreichen nach Möglichkeit ca. 80% Unit-Test-Coverage (mit Augenmaß).
- README enthält „Run“ und „Test“ Schritte.
