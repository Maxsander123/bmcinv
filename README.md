# bmcinv - BMC Inventory Tool

[![Go](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Ein Read-Only Hardware-Inventarisierungs-Tool für Rechenzentren, das via Redfish API (iDRAC, iLO, IPMI) Server abfragt und Hardware-Details in einer lokalen SQLite-Datenbank cached.

## Features

- 🔍 **Schnelle Suche** - Finde Server anhand von RAM-Seriennummern, MAC-Adressen, Disk-Serials
- 🔐 **Smart Credentials** - Verschiedene Passwörter für verschiedene BMC-Typen (Dell iDRAC, HPE iLO, IPMI)
- ⚡ **Paralleles Scanning** - Worker-Pool für schnelles Scannen großer IP-Bereiche
- 💾 **SQLite Cache** - Sekundenschnelle Suche ohne Netzwerkzugriff
- 📊 **CSV/JSON Export** - Exportiere Inventar für Excel oder andere Tools
- 🔗 **Command Chaining** - Verkette Scan und Export in einem Befehl
- 📖 **Man Page** - Vollständige Dokumentation via `man bmcinv`
- 🐚 **Shell Completion** - Autovervollständigung für Bash, Zsh, Fish

## Installation

### Option 1: go install (empfohlen)
```bash
go install github.com/Maxsander123/bmcinv@latest
```

### Option 2: Von Source
```bash
git clone https://github.com/Maxsander123/bmcinv.git
cd bmcinv
make install   # Installiert Binary + Man Page (braucht sudo)
```

### Option 3: Nur für aktuellen User
```bash
make install-user  # Installiert nach ~/.local/bin
```

### Shell-Completion installieren
```bash
# Nach der Installation:
make install-completion  # Bash completion

# Oder manuell für andere Shells:
bmcinv completion bash > ~/.bashrc.d/bmcinv
bmcinv completion zsh > ~/.zshrc.d/_bmcinv
bmcinv completion fish > ~/.config/fish/completions/bmcinv.fish
```

## Schnellstart

```bash
# Status anzeigen
bmcinv status

# Server scannen
bmcinv scan 192.168.1.100

# Ganzes Subnetz scannen  
bmcinv scan 192.168.1.0/24

# Scannen UND direkt als CSV exportieren
bmcinv scan 192.168.1.0/24 --export -o ~/Desktop

# Nach MAC-Adresse suchen
bmcinv find 00:1B:21:AB:CD:EF

# Man Page lesen
man bmcinv
```

## Befehle

| Befehl | Beschreibung |
|--------|--------------|
| `bmcinv scan <cidr\|ip>` | Server via BMC scannen |
| `bmcinv find <string>` | Suche in allen Tabellen |
| `bmcinv export` | Export zu CSV/JSON |
| `bmcinv status` | Übersicht anzeigen |
| `bmcinv completion` | Shell-Completion generieren |

## Verwendung

### Scannen

```bash
# Einzelnen Server scannen
bmcinv scan 192.168.1.100

# Ganzes Subnetz scannen
bmcinv scan 192.168.1.0/24

# Mit mehr Workern (schneller)
bmcinv scan 10.0.0.0/24 -w 20

# Timeout pro Host anpassen
bmcinv scan 10.0.0.0/24 -t 60

# Scannen und direkt exportieren (Command Chaining)
bmcinv scan 10.0.0.0/24 --export
bmcinv scan 10.0.0.0/24 -e -o ~/backup
bmcinv scan 10.0.0.0/24 -e -F json -o /tmp/inventory

# Alle Optionen kombinieren
bmcinv scan 10.0.0.0/24 -w 20 -t 60 -e -F csv -o ~/backup
```

### Suchen

```bash
# Nach MAC-Adresse suchen (alle Formate werden erkannt)
bmcinv find 00:1B:21:AB:CD:EF
bmcinv find 00-1B-21-AB-CD-EF
bmcinv find 001B21ABCDEF

# Nach RAM-Seriennummer suchen  
bmcinv find MEM12345678

# Alle Samsung-DIMMs finden
bmcinv find Samsung --type memory

# Alle Dell-Server finden
bmcinv find "Dell Inc."

# Exakte Suche (keine Wildcards)
bmcinv find MEM12345678 --exact

# Suche auf bestimmten Komponenten-Typ einschränken
bmcinv find Samsung --type memory    # Nur RAM
bmcinv find Seagate --type storage   # Nur Storage
bmcinv find "10.0.0" --type network  # Nur Netzwerk
bmcinv find "Dell" --type server     # Nur Server

# Anzahl der Ergebnisse begrenzen
bmcinv find Samsung -l 10

# Verbose-Ausgabe für Details
bmcinv -v find MEM12345678
```

> **Hinweis:** Die Suche verwendet standardmäßig Wildcards (z.B. findet `Samsung` auch `Samsung Inc.`).
> Mit `--exact` wird nur eine exakte Übereinstimmung zurückgegeben.

> **MAC-Adressen:** Das Tool normalisiert automatisch alle MAC-Formate (`AA:BB:CC:DD:EE:FF`,
> `AA-BB-CC-DD-EE-FF`, `AABBCCDDEEFF`) auf das interne Format `AA:BB:CC:DD:EE:FF`.

### Exportieren

```bash
# Als CSV exportieren (Standard)
bmcinv export

# In bestimmtes Verzeichnis
bmcinv export -o ~/Desktop

# Als JSON exportieren
bmcinv export -f json -o ~/inventory.json

# CSV in Backup-Ordner exportieren
bmcinv export -f csv -o ~/backup
```

## Konfiguration

Die Config-Datei wird automatisch erstellt unter `~/.bmcinv/config.yaml`:

```yaml
# Credentials für verschiedene BMC-Typen
credentials:
  idrac:
    username: root
    password: calvin
  ilo:
    username: Administrator
    password: your-password
  ipmi:
    username: ADMIN
    password: ADMIN

# Scanner-Einstellungen
scan:
  workers: 10          # Parallele Verbindungen
  timeout_secs: 30     # Timeout pro Host
  retry_attempts: 2    # Wiederholungsversuche

# Datenbank
database:
  path: ~/.bmcinv/inventory.db
```

## Architektur

```
bmcinv/
├── main.go                    # Entry Point
├── man/bmcinv.1              # Man Page
├── cmd/                       # CLI Commands (Cobra)
│   ├── root.go
│   ├── scan.go
│   ├── find.go
│   ├── export.go
│   └── status.go
└── internal/
    ├── config/               # Viper Config + Smart Credentials
    ├── database/            # GORM/SQLite Layer
    ├── models/              # Server, Memory, Storage, Network
    ├── scanner/             # Worker-Pool + Vendor Detection
    └── finder/              # Parallele Multi-Table Suche
```

## Beispiel-Output

### Scan
```
$ bmcinv scan 192.168.1.0/28 --export -o ~/backup

Starting scan of 192.168.1.0/28...

=== Scan Complete ===
Duration:   2.453s
Total:      14 hosts
Successful: 12
Failed:     2

=== Successfully Scanned Servers ===
IP             VENDOR      MODEL           SERIAL
192.168.1.1    Dell Inc.   PowerEdge R640  CHS7XK2ABC
192.168.1.2    HPE         ProLiant DL380  MXQ1234567
192.168.1.3    Dell Inc.   PowerEdge R740  CHSABC1234
...

--- Exporting data ---
✓ Exported 12 servers to /home/user/backup/servers_20260310_120000.csv
✓ Exported 384 memory modules to /home/user/backup/memory_20260310_120000.csv
✓ Exported 96 storage devices to /home/user/backup/storage_20260310_120000.csv
✓ Exported 48 network interfaces to /home/user/backup/networks_20260310_120000.csv
```

### Find
```
$ bmcinv find Samsung --type memory

=== Found 24 Result(s) ===

─── MEMORY (24) ───
SERVER IP      COMPONENT                          MATCHED        VALUE
192.168.1.1    Slot: DIMM_A1, 64GB Samsung DDR4   manufacturer   Samsung
192.168.1.1    Slot: DIMM_A2, 64GB Samsung DDR4   manufacturer   Samsung
192.168.1.2    Slot: DIMM_B1, 32GB Samsung DDR4   manufacturer   Samsung
...
```

## Datenbank-Schema

| Tabelle | Felder |
|---------|--------|
| **servers** | IP, Vendor, Model, ChassisSerial, BiosVersion, BMCVersion, Hostname, LastScanned |
| **memory** | Slot, CapacityGB, Speed, Type, Manufacturer, PartNumber, SerialNumber, Health |
| **storage** | Slot, MediaType, Protocol, CapacityGB, Manufacturer, Model, SerialNumber, FirmwareRev, Health, PredFailure |
| **networks** | Port, MacAddress, IPAddress, LinkStatus, LinkSpeedMbps, Manufacturer, Model, FirmwareRev |

## CSV Export Spalten

Beim CSV-Export werden **4 Dateien** erstellt. Jede Komponenten-Datei enthält die Server-Informationen als Kontext:

**`servers_<timestamp>.csv`**
```
IP, Vendor, Model, ChassisSerial, BiosVersion, BMCVersion, Hostname, LastScanned
```

**`memory_<timestamp>.csv`**
```
ServerIP, ServerSerial, ServerVendor, ServerModel, ServerHostname,
Slot, CapacityGB, Speed, Type, Manufacturer, PartNumber, SerialNumber, Health
```

**`storage_<timestamp>.csv`**
```
ServerIP, ServerSerial, ServerVendor, ServerModel, ServerHostname,
Slot, MediaType, Protocol, CapacityGB, Manufacturer, Model, SerialNumber, FirmwareRev, Health, PredFailure
```

**`networks_<timestamp>.csv`**
```
ServerIP, ServerSerial, ServerVendor, ServerModel, ServerHostname,
Port, MacAddress, IPAddress, LinkStatus, LinkSpeedMbps, Manufacturer, Model, FirmwareRev
```

> **Hinweis:** `PredFailure` in storage.csv ist `true`, wenn SMART-Daten einen bevorstehenden Ausfall vorhersagen.

## Datenbank verwalten

```bash
# Lokalen Cache löschen (wird beim nächsten Scan neu aufgebaut)
rm ~/.bmcinv/inventory.db

# Datenbank sichern
cp ~/.bmcinv/inventory.db ~/inventory-backup-$(date +%Y%m%d).db

# Direkte SQLite-Abfragen (optional)
sqlite3 ~/.bmcinv/inventory.db ".tables"
sqlite3 ~/.bmcinv/inventory.db "SELECT COUNT(*) FROM servers;"
sqlite3 ~/.bmcinv/inventory.db "SELECT ip, vendor, model FROM servers;"

# Alle Server mit vorhergesagten Festplattenausfällen
sqlite3 ~/.bmcinv/inventory.db "SELECT s.ip, st.slot, st.model FROM storage st JOIN servers s ON st.server_id = s.id WHERE st.pred_failure = 1;"
```

## Performance-Tipps

```bash
# Standard: 10 Worker, 30s Timeout
bmcinv scan 192.168.1.0/24

# Für größere Subnetze: mehr Worker, längerer Timeout
bmcinv scan 10.0.0.0/16 -w 50 -t 60

# Oder dauerhaft in der Config anpassen:
# scan:
#   workers: 50
#   timeout_secs: 60
#   retry_attempts: 3

# Mehrere Subnetze nacheinander scannen
for net in 10.0.0.0/24 10.0.1.0/24 10.0.2.0/24; do
  bmcinv scan "$net"
done
```

Automatischen Export einrichten (täglich um 02:00 Uhr mit `crontab -e`):
```
0 2 * * * bmcinv scan 10.0.0.0/16 --export -o /backup/inventory
```

## Troubleshooting

### Häufige Fehler

| Fehler | Ursache | Lösung |
|--------|---------|--------|
| `unable to detect BMC type` | Host nicht erreichbar oder kein BMC | CIDR/IP prüfen, Netzwerk-Konnektivität testen |
| `credential error` | Keine Credentials konfiguriert | `~/.bmcinv/config.yaml` prüfen |
| `database initialization failed` | Keine Schreibrechte | `chmod 700 ~/.bmcinv/` ausführen |
| `No data to export` | Noch kein Scan durchgeführt | Zuerst `bmcinv scan <ip>` ausführen |

### Debug-Modus

```bash
# Verbose-Ausgabe für alle Befehle
bmcinv -v scan 192.168.1.100
bmcinv -v find Samsung
bmcinv -v export

# Status prüfen
bmcinv status
```

## Dokumentation

```bash
# Man Page anzeigen
man bmcinv

# Hilfe für einzelne Befehle
bmcinv scan --help
bmcinv find --help
bmcinv export --help
```

## Entwicklung

```bash
# Projekt klonen
git clone https://github.com/Maxsander123/bmcinv.git
cd bmcinv

# Dependencies installieren
go mod tidy

# Bauen
make build

# Tests ausführen
make test

# Lokal installieren
make install
```

## Sicherheit

- Credentials werden in `~/.bmcinv/config.yaml` gespeichert
- Stelle sicher, dass die Datei nur für dich lesbar ist: `chmod 600 ~/.bmcinv/config.yaml`
- Das Tool führt **nur Lesezugriffe** auf BMC-Interfaces durch
- Der Pre-Flight-Check (Vendor-Erkennung) ist nicht authentifiziert; Datenabruf erfolgt mit Vendor-spezifischen Credentials

```bash
# Empfohlene Dateiberechtigungen
chmod 700 ~/.bmcinv/
chmod 600 ~/.bmcinv/config.yaml

# Credentials nie in Versionskontrolle einchecken!
# Globale gitignore-Datei (~/.gitignore_global) ergänzen:
echo ".bmcinv/" >> ~/.gitignore_global
git config --global core.excludesfile ~/.gitignore_global
```

## License

MIT

## Links

- **Repository:** https://github.com/Maxsander123/bmcinv
- **Issues:** https://github.com/Maxsander123/bmcinv/issues
