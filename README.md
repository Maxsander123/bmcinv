# bmcinv – BMC Inventory Tool

[![Go](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![CI](https://github.com/Maxsander123/bmcinv/actions/workflows/ci.yml/badge.svg)](https://github.com/Maxsander123/bmcinv/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**bmcinv** ist ein Read-Only Hardware-Inventarisierungs-Tool für Rechenzentren. Es fragt Server über ihre BMC-Schnittstellen (Dell iDRAC, HPE iLO, IPMI-kompatible BMCs) via **Redfish API v1** ab und cached die Hardware-Details in einer lokalen SQLite-Datenbank. Suchen laufen lokal – ohne erneuten Netzwerkzugriff.

---

## Inhaltsverzeichnis

1. [Features](#features)
2. [Schnellstart](#schnellstart)
3. [Installation](#installation)
4. [Befehle & Flags (Referenz)](#befehle--flags-referenz)
   - [scan](#scan)
   - [find](#find)
   - [export](#export)
   - [status](#status)
5. [Konfiguration](#konfiguration)
   - [Passwörter via Umgebungsvariablen](#passwörter-via-umgebungsvariablen)
   - [Vollständige Config-Referenz](#vollständige-config-referenz)
6. [Sicherheit](#sicherheit)
7. [Wie es funktioniert](#wie-es-funktioniert)
   - [Vendor-Erkennung (Pre-Flight)](#vendor-erkennung-pre-flight)
   - [Redfish Daten-Sammlung](#redfish-daten-sammlung)
   - [Worker-Pool](#worker-pool)
   - [Datenbank](#datenbank)
8. [Datenbankschema](#datenbankschema)
9. [Architektur](#architektur)
10. [Troubleshooting](#troubleshooting)
11. [Entwicklung](#entwicklung)
12. [Bekannte Einschränkungen](#bekannte-einschränkungen)

---

## Features

- **Echte Redfish API** – Fragt iDRAC 7+, iLO 4+, Supermicro X10+, ASUS ASMB und generische BMCs ab
- **Automatische Vendor-Erkennung** – Erkennt Dell / HPE / Supermicro / ASUS / MSI ohne manuelle Angabe
- **Paralleles Scanning** – Konfigurierbarer Worker-Pool für schnelles Scannen ganzer IP-Bereiche
- **Smart Credentials** – Separate Passwörter pro BMC-Typ; Übergabe via Env-Variable möglich
- **Lokaler SQLite-Cache** – Suchen laufen in Millisekunden, ohne Netzwerk
- **Komponentensuche** – Suche nach Seriennummern von RAM, Disks, NICs; MAC-Adressen in jedem Format
- **CSV/JSON Export** – Timestamped-Dateien pro Tabelle; direkt aus `scan` heraus verkettbar
- **Shell-Completion** – Bash, Zsh, Fish
- **Man Page** – `man bmcinv`

---

## Schnellstart

```bash
# 1. Config und DB werden beim ersten Aufruf automatisch angelegt
bmcinv status

# 2. Passwörter setzen (sicherer als config.yaml)
export BMCINV_IDRAC_PASSWORD="meinPasswort"
export BMCINV_ILO_PASSWORD="meinPasswort"

# 3. Einzelnen Server scannen
bmcinv scan 10.0.0.100

# 4. Ganzes Subnetz scannen (14 Hosts, /28)
bmcinv scan 10.0.0.0/28

# 5. Scannen und sofort als CSV exportieren
bmcinv scan 10.0.0.0/24 --export -o ~/backup

# 6. Suchen
bmcinv find 00:1B:21:AB:CD:EF     # nach MAC
bmcinv find MEM12345678            # nach RAM-Seriennummer
bmcinv find Samsung --type memory  # alle Samsung-DIMMs
```

---

## Installation

### Option 1: go install

```bash
go install github.com/Maxsander123/bmcinv@latest
```

Das Binary landet in `$(go env GOPATH)/bin/bmcinv` – stelle sicher, dass dieser Pfad in `$PATH` liegt.

### Option 2: Aus dem Quellcode

```bash
git clone https://github.com/Maxsander123/bmcinv.git
cd bmcinv
make install        # Binary + Man Page systemweit (braucht sudo)
```

### Option 3: Nur für aktuellen User (kein sudo)

```bash
make install-user   # Installiert nach ~/.local/bin
```

Stelle sicher, dass `~/.local/bin` in deinem `$PATH` ist:

```bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

### Shell-Completion

```bash
# Nach der Installation einmalig einrichten:
make install-completion           # Bash (systemweit)

# Manuell für andere Shells:
bmcinv completion bash  >> ~/.bashrc
bmcinv completion zsh   > "${fpath[1]}/_bmcinv"
bmcinv completion fish  > ~/.config/fish/completions/bmcinv.fish
```

---

## Befehle & Flags (Referenz)

### Globale Flags

Diese Flags können vor jedem Subcommand angegeben werden:

| Flag | Kurz | Standard | Beschreibung |
|------|------|---------|--------------|
| `--config` | – | `~/.bmcinv/config.yaml` | Alternativer Pfad zur Config-Datei |
| `--verbose` | `-v` | `false` | Ausführliche Ausgabe (inkl. fehlgeschlagener Hosts, SQL-Queries) |

---

### scan

Scannt einen einzelnen Host oder einen IP-Bereich via Redfish API.

```
bmcinv scan <cidr|ip> [flags]
```

**Argumente:**

| Argument | Beschreibung |
|----------|-------------|
| `<cidr\|ip>` | Einzelne IP (`10.0.0.1`) oder CIDR-Bereich (`10.0.0.0/24`). Netzwerk- und Broadcast-Adresse werden automatisch übersprungen. |

**Flags:**

| Flag | Kurz | Standard | Beschreibung |
|------|------|---------|--------------|
| `--workers` | `-w` | aus config | Anzahl paralleler Worker-Goroutinen |
| `--timeout` | `-t` | aus config | Timeout pro Host in Sekunden |
| `--export` | `-e` | `false` | Daten direkt nach dem Scan exportieren |
| `--export-dir` | `-o` | `.` | Ausgabeverzeichnis für den Export |
| `--export-format` | `-F` | `csv` | Exportformat: `csv` oder `json` |

**Beispiele:**

```bash
# Einzelnen Host scannen
bmcinv scan 192.168.1.100

# Ganzes /24 Subnetz mit 20 Workern
bmcinv scan 10.0.0.0/24 -w 20

# Scannen und als JSON exportieren
bmcinv scan 10.0.0.0/24 -e -F json -o /tmp/inventory

# Verbose: zeigt auch fehlgeschlagene Hosts
bmcinv scan 10.0.0.0/28 -v
```

**Scan-Ablauf pro Host:**

```
1. GET https://<ip>/redfish/v1/  →  Vendor-Erkennung (kein Auth)
2. Credentials aus Config laden (nach Vendor)
3. Redfish-API abfragen (mit Basic Auth):
   • /redfish/v1/Systems/{id}                      → Server-Basisinfos
   • /redfish/v1/Systems/{id}/Memory/*             → RAM-Module
   • /redfish/v1/Systems/{id}/Storage/*/Drives/*   → Disks
   • /redfish/v1/Systems/{id}/EthernetInterfaces/* → NICs
   • /redfish/v1/Managers/{id}                     → BMC-Firmware-Version
4. Upsert in SQLite (nach IP-Adresse)
```

**Ausgabe-Beispiel:**

```
Starting scan of 10.0.0.0/28...

=== Scan Complete ===
Duration:   4.213s
Total:      14 hosts
Successful: 11
Failed:     3

=== Successfully Scanned Servers ===
IP           VENDOR      MODEL           SERIAL
10.0.0.1     Dell Inc.   PowerEdge R740  CNSABC1234
10.0.0.2     HPE         ProLiant DL380  MXQ9876543
10.0.0.3     Dell Inc.   PowerEdge R640  CNSXYZ9876
...
```

Mit `-v` werden zusätzlich die fehlgeschlagenen Hosts ausgegeben:

```
=== Failed Hosts ===
  10.0.0.10: no Redfish endpoint reachable
  10.0.0.11: credential error: no credentials configured for BMC type: unknown
  10.0.0.14: data collection failed after 3 attempts: HTTP 401
```

---

### find

Durchsucht die lokale Datenbank parallel über alle Tabellen.

```
bmcinv find <suchbegriff> [flags]
```

**Argumente:**

| Argument | Beschreibung |
|----------|-------------|
| `<suchbegriff>` | Beliebige Zeichenkette. MAC-Adressen werden in alle Formate normalisiert. Wildcards werden automatisch gesetzt (Teilsuche). |

**Flags:**

| Flag | Kurz | Standard | Beschreibung |
|------|------|---------|--------------|
| `--exact` | `-e` | `false` | Exaktes Match, keine Wildcards |
| `--limit` | `-l` | `100` | Maximale Treffer pro Tabelle |
| `--type` | `-T` | `""` | Nur in diesem Typ suchen: `server`, `memory`, `storage`, `network` |

**Durchsuchte Felder:**

| Tabelle | Felder |
|---------|--------|
| `servers` | IP, Hostname, Chassis-Seriennummer, Vendor, Model |
| `memory` | Seriennummer, Hersteller, Teilenummer |
| `storage` | Seriennummer, Model, Hersteller |
| `networks` | MAC-Adresse, IP-Adresse |

**MAC-Adress-Normalisierung:**

Das Tool erkennt MAC-Adressen automatisch und normalisiert sie vor der Suche:

```bash
bmcinv find 001b21abcdef       # ohne Trenner
bmcinv find 00:1b:21:ab:cd:ef  # mit Doppelpunkt
bmcinv find 00-1b-21-ab-cd-ef  # mit Bindestrich
# alle drei suchen nach: 00:1B:21:AB:CD:EE:FF
```

**Beispiele:**

```bash
# RAM nach Seriennummer suchen
bmcinv find MEM12345678

# Disk nach Seriennummer
bmcinv find DSK9876543210

# Alle NVMe-Disks von Samsung
bmcinv find Samsung --type storage

# Alle Dell-Server
bmcinv find "Dell Inc." --type server

# Exakte IP-Suche
bmcinv find 10.0.0.42 --exact

# MAC in beliebigem Format
bmcinv find 001B21ABCDEF
```

**Ausgabe-Beispiel:**

```
=== Found 3 Result(s) ===

─── MEMORY (3) ───
SERVER IP    COMPONENT                         MATCHED         VALUE
10.0.0.1     Slot: DIMM_A1, 64GB Samsung DDR4  manufacturer    Samsung
10.0.0.1     Slot: DIMM_A2, 64GB Samsung DDR4  manufacturer    Samsung
10.0.0.2     Slot: DIMM_B1, 32GB Samsung DDR4  manufacturer    Samsung
```

---

### export

Exportiert alle gespeicherten Inventardaten aus der Datenbank.

```
bmcinv export [flags]
```

**Flags:**

| Flag | Kurz | Standard | Beschreibung |
|------|------|---------|--------------|
| `--format` | `-f` | `csv` | Ausgabeformat: `csv` oder `json` |
| `--output` | `-o` | `.` | Ausgabeverzeichnis (CSV) oder Dateiname (JSON) |

**CSV-Export:**

Erstellt vier Dateien mit Zeitstempel im Ausgabeverzeichnis:

```
servers_20260918_143022.csv
memory_20260918_143022.csv
storage_20260918_143022.csv
networks_20260918_143022.csv
```

Jede Zeile in den Komponenten-Dateien enthält den Server-Kontext (IP, Seriennummer, Vendor, Model, Hostname), sodass die CSV-Dateien ohne weitere JOINs direkt in Excel verwendbar sind.

**CSV-Spalten:**

| Datei | Spalten |
|-------|---------|
| `servers_*.csv` | IP, Vendor, Model, ChassisSerial, BiosVersion, BMCVersion, Hostname, LastScanned |
| `memory_*.csv` | ServerIP, ServerSerial, ServerVendor, ServerModel, ServerHostname, Slot, CapacityGB, Speed, Type, Manufacturer, PartNumber, SerialNumber, Health |
| `storage_*.csv` | ServerIP, ServerSerial, ServerVendor, ServerModel, ServerHostname, Slot, MediaType, Protocol, CapacityGB, Manufacturer, Model, SerialNumber, FirmwareRev, Health, PredFailure |
| `networks_*.csv` | ServerIP, ServerSerial, ServerVendor, ServerModel, ServerHostname, Port, MacAddress, IPAddress, LinkStatus, LinkSpeedMbps, Manufacturer, Model, FirmwareRev |

**JSON-Export:**

Erstellt eine einzelne Datei mit allen Servern und verschachtelten Komponenten:

```json
[
  {
    "id": 1,
    "ip": "10.0.0.1",
    "vendor": "Dell Inc.",
    "model": "PowerEdge R740",
    "chassis_serial": "CNSABC1234",
    "bios_version": "2.14.0",
    "bmc_version": "5.10.30.20",
    "hostname": "server01.dc1.example.com",
    "last_scanned": "2026-09-18T14:30:22Z",
    "memory": [ ... ],
    "storage": [ ... ],
    "networks": [ ... ]
  }
]
```

**Beispiele:**

```bash
# CSV ins aktuelle Verzeichnis
bmcinv export

# CSV in ein Backup-Verzeichnis
bmcinv export -o ~/backup/dc1

# JSON in eine Datei
bmcinv export -f json -o /tmp/inventory.json

# JSON in ein Verzeichnis (Dateiname wird automatisch gesetzt)
bmcinv export -f json -o /tmp/
```

---

### status

Zeigt Konfiguration, Credentials und Datenbankstatistiken.

```
bmcinv status
```

**Ausgabe-Beispiel:**

```
=== BMC Inventory Status ===

Configuration:
  Config file:  /home/user/.bmcinv/config.yaml
  Database:     /home/user/.bmcinv/inventory.db
  Workers:      10
  Timeout:      30s

Configured Credentials:
  idrac: user=root
  ilo:   user=Administrator
  ipmi:  user=ADMIN

Inventory Statistics:
  Servers:      47
  Memory DIMMs: 1504
  Storage:      376
  NICs:         188
```

Passwörter werden nie angezeigt.

---

## Konfiguration

Beim ersten Start wird automatisch eine Config-Datei unter `~/.bmcinv/config.yaml` angelegt. Das Verzeichnis hat Rechte `0750`, die Config-Datei `0600`.

### Passwörter via Umgebungsvariablen

Die sicherste Methode — die Config-Datei enthält dann keine sensiblen Daten:

```bash
export BMCINV_IDRAC_USERNAME="root"
export BMCINV_IDRAC_PASSWORD="meinDellPasswort"       # Dell iDRAC

export BMCINV_ILO_USERNAME="Administrator"
export BMCINV_ILO_PASSWORD="meinHPEPasswort"          # HPE iLO

export BMCINV_SUPERMICRO_USERNAME="ADMIN"
export BMCINV_SUPERMICRO_PASSWORD="meinSMCPasswort"   # Supermicro

export BMCINV_IPMI_USERNAME="ADMIN"
export BMCINV_IPMI_PASSWORD="meinPasswort"            # ASUS / MSI / unbekannte BMCs
```

Env-Variablen haben Vorrang vor der Config-Datei. In CI/CD-Systemen können diese als Secrets gesetzt werden.

### Vollständige Config-Referenz

```yaml
# ~/.bmcinv/config.yaml

# Credentials für verschiedene BMC-Typen
# Tipp: Passwörter lieber via Umgebungsvariablen setzen (s. oben)
credentials:
  idrac:          # Dell PowerEdge (iDRAC 7+)
    username: root
    password: ""          # default: calvin — oder BMCINV_IDRAC_PASSWORD
  ilo:            # HPE ProLiant (iLO 4+)
    username: Administrator
    password: ""          # oder: BMCINV_ILO_PASSWORD
  supermicro:     # Supermicro (X10/X11/X12/H12/H13)
    username: ADMIN
    password: ""          # default: ADMIN — oder BMCINV_SUPERMICRO_PASSWORD
  ipmi:           # Generisch: ASUS ASMB, MSI, unbekannte BMCs
    username: ADMIN
    password: ""          # oder: BMCINV_IPMI_PASSWORD

# SQLite-Datenbankpfad
database:
  path: ~/.bmcinv/inventory.db

# Scanner-Einstellungen
scan:
  # Anzahl paralleler Worker-Goroutinen.
  # Faustregel: 10–20 für normale Netze, max. 50 (höhere Werte können BMCs überlasten).
  workers: 10

  # Timeout pro Host in Sekunden (inkl. TLS-Handshake und alle API-Abfragen).
  timeout_secs: 30

  # Anzahl Wiederholungsversuche bei temporären Fehlern (linearer Backoff: 1s, 2s, 3s…).
  retry_attempts: 2

  # TLS-Zertifikatsvalidierung überspringen.
  # Standard: true – fast alle BMCs nutzen selbst-signierte Zertifikate.
  # Nur auf false setzen, wenn deine BMCs Zertifikate von einer vertrauenswürdigen CA haben.
  tls_skip_verify: true
```

### Alternativer Config-Pfad

```bash
bmcinv --config /etc/bmcinv/prod.yaml scan 10.0.0.0/24
```

---

## Sicherheit

### Credentials-Speicherung

| Methode | Sicherheit | Empfehlung |
|---------|-----------|------------|
| Env-Variablen (`BMCINV_*_PASSWORD`) | Hoch | Empfohlen für Produktion und CI/CD |
| Config-Datei (`~/.bmcinv/config.yaml`) | Mittel | Nur mit `chmod 600`, kein Backup-Sync |
| Klartext in Shell-Skripten | Niedrig | Vermeiden |

### TLS

BMCs in Rechenzentren nutzen fast ausnahmslos selbst-signierte TLS-Zertifikate. `tls_skip_verify: true` ist daher der Standardwert. Das bedeutet: Der Traffic ist verschlüsselt, aber die Identität des BMC wird nicht kryptografisch verifiziert. Dies ist in isolierten Management-Netzwerken akzeptabel.

Wenn deine Umgebung PKI-verwaltete BMC-Zertifikate hat, setze `tls_skip_verify: false`.

### Read-Only

Das Tool sendet ausschließlich HTTP-GET-Requests an die Redfish API. Es werden keinerlei Konfigurationsänderungen an Servern vorgenommen.

### Datenbankdatei

`~/.bmcinv/inventory.db` enthält Hardware-Inventardaten (keine Credentials). Sie kann aber Hostnamen und IP-Adressen enthalten – entsprechend schützen (kein Git-Commit, kein unsicheres Cloud-Backup). Die Datei ist durch `.gitignore` ausgeschlossen.

---

## Wie es funktioniert

### Vendor-Erkennung (Pre-Flight)

Für jede zu scannende IP sendet bmcinv zunächst einen unauthentifizierten GET-Request an den Redfish-Root-Endpunkt:

```
GET https://<ip>/redfish/v1/
```

Anhand von HTTP-Response-Header (`Server:`) und Response-Body wird der BMC-Typ erkannt:

| Erkennungsmerkmal | Vendor | Credential-Typ |
|-------------------|--------|----------------|
| `Server: iDRAC/…` im HTTP-Header | Dell iDRAC | `idrac` |
| `Server: iLO/…` im HTTP-Header | HPE iLO | `ilo` |
| `"Dell"` / `"iDRAC"` im Response-Body | Dell iDRAC | `idrac` |
| `"Hewlett"` / `"HPE"` / `"iLO"` im Body | HPE iLO | `ilo` |
| `"Supermicro"` im Body | Supermicro | `supermicro` |
| `"ASUS"` / `"ASRockRack"` im Body | ASUS / ASRock Rack | `ipmi` |
| Antwort vorhanden, Vendor unbekannt (MSI usw.) | Generisch | `ipmi` |
| Keine Antwort / Timeout | – | übersprungen |

**HPE iLO 4 (Gen8/Gen9)**: iLO 4 implementiert kein Standard-Redfish-Storage. Das Tool erkennt leere Ergebnisse und fällt automatisch auf den proprietären HPE SmartStorage-Pfad zurück (`/SmartStorage/ArrayControllers/`). iLO 5+ (Gen10+) nutzt den Standard-Pfad.

Erst nach der Erkennung werden die passenden Credentials geladen und authenticated Requests gesendet.

### Redfish Daten-Sammlung

Die Daten-Sammlung folgt standardisierten Redfish-v1-Endpunkten. Der Pfad wird dynamisch aus dem Root-Dokument ermittelt – das Tool ist damit nicht auf herstellerspezifische Fest-Pfade angewiesen:

```
1. GET /redfish/v1/
   → Liefert @odata.id-Links zu Systems und Managers

2. GET <systems-link>/
   → Liste aller Systeme (meist nur eines)

3. GET <system-link>
   → Vendor, Model, Seriennummer, BIOS-Version, Hostname

4. GET <system>/Memory/
   → Liste aller DIMM-Slots
   GET <system>/Memory/{slot}
   → CapacityMiB, Speed, Type, Hersteller, Seriennummer, Health
   (leere Slots: CapacityMiB == 0 → übersprungen)

5. GET <system>/Storage/
   → Liste aller Storage-Controller
   GET <system>/Storage/{ctrl}
   → Drives-Liste
   GET <system>/Storage/{ctrl}/Drives/{drive}
   → MediaType, Protocol, Kapazität, Hersteller, Seriennummer, Health, FailurePredicted

6. GET <system>/EthernetInterfaces/
   → Liste aller Netzwerkkarten
   GET <system>/EthernetInterfaces/{id}
   → MAC-Adresse, IPv4, LinkStatus, Speed

7. GET <managers-link>/
   → Liste aller Manager (BMC-Instanzen)
   GET <manager>
   → FirmwareVersion (BMC-Firmware)
```

Fehler bei einzelnen Endpunkten führen nicht zum Abbruch – das Tool sammelt, was es bekommt.

### Worker-Pool

Der Scanner verwendet das Fan-Out/Fan-In-Pattern mit konfigurierbarer Parallelität. Pro `ScanCIDR`-Aufruf wird ein eigener Result-Channel erstellt, sodass der Scanner mehrfach verwendet werden kann:

```
ScanCIDR("10.0.0.0/24")
  │
  ├── Erstellt jobs-Channel (alle IPs) + results-Channel
  │
  ├── Startet N Worker-Goroutinen (fan-out)
  │   Jeder Worker:
  │     for ip := range jobs {
  │       results <- scanHost(ip)   // vendor detect + redfish + db
  │     }
  │
  ├── Feeder-Goroutine: schreibt IPs in jobs, schließt jobs danach
  │
  └── Wartet auf alle Worker (wg.Wait()), schließt results
      → Collector: liest results bis Channel geschlossen
```

Context-Cancellation (`scanner.Cancel()`) beendet alle Worker sauber. Der Retry-Backoff beachtet ebenfalls den Context.

### Datenbank

SQLite läuft im WAL-Modus (Write-Ahead Logging) für bessere Parallelität. GORM übernimmt:

- **AutoMigrate** beim Start (Schema-Updates ohne Datenverlust)
- **Upsert nach IP**: Existierende Server werden aktualisiert, neue angelegt
- **Transaktionale Komponenten-Ersetzung**: Alle Komponenten eines Servers werden atomar gelöscht und neu geschrieben (saubere Slate nach jedem Scan)
- **Custom-Indizes** auf Seriennummern und MAC-Adressen für schnelle `find`-Abfragen

---

## Datenbankschema

### Tabelle: `servers`

| Spalte | Typ | Beschreibung |
|--------|-----|-------------|
| `id` | INTEGER | Primärschlüssel |
| `ip` | TEXT | IP-Adresse des BMC (Unique Index) |
| `vendor` | TEXT | Hersteller (z. B. "Dell Inc.", "HPE") |
| `model` | TEXT | Server-Modell (z. B. "PowerEdge R740") |
| `chassis_serial` | TEXT | Chassis-Seriennummer |
| `bios_version` | TEXT | BIOS/UEFI-Version |
| `bmc_version` | TEXT | BMC-Firmware-Version |
| `hostname` | TEXT | Hostname laut Redfish |
| `last_scanned` | DATETIME | Zeitpunkt des letzten Scans |
| `created_at` | DATETIME | Ersterfassung |
| `updated_at` | DATETIME | Letzte Aktualisierung |

### Tabelle: `memory`

| Spalte | Typ | Beschreibung |
|--------|-----|-------------|
| `server_id` | INTEGER | FK → servers.id (CASCADE DELETE) |
| `slot` | TEXT | DIMM-Slot (z. B. "DIMM_A1", "P1-DIMMA1") |
| `capacity_gb` | INTEGER | Kapazität in GB |
| `speed` | INTEGER | Taktfrequenz in MHz |
| `type` | TEXT | RAM-Typ ("DDR4", "DDR5") |
| `manufacturer` | TEXT | Hersteller |
| `part_number` | TEXT | Teilenummer |
| `serial_number` | TEXT | Seriennummer (RMA-Tracking) |
| `health` | TEXT | OK, Warning, Critical |

### Tabelle: `storage`

| Spalte | Typ | Beschreibung |
|--------|-----|-------------|
| `server_id` | INTEGER | FK → servers.id (CASCADE DELETE) |
| `slot` | TEXT | Bay-Bezeichnung (z. B. "Bay 1") |
| `media_type` | TEXT | SSD, HDD, NVMe |
| `protocol` | TEXT | SATA, SAS, NVMe |
| `capacity_gb` | INTEGER | Kapazität in GB |
| `manufacturer` | TEXT | Hersteller |
| `model` | TEXT | Laufwerk-Modell |
| `serial_number` | TEXT | Seriennummer (RMA-Tracking) |
| `firmware_rev` | TEXT | Firmware-Version |
| `health` | TEXT | OK, Warning, Critical, Failed |
| `pred_failure` | BOOLEAN | SMART Predicted Failure |

### Tabelle: `networks`

| Spalte | Typ | Beschreibung |
|--------|-----|-------------|
| `server_id` | INTEGER | FK → servers.id (CASCADE DELETE) |
| `port` | TEXT | Port-Bezeichnung (z. B. "NIC.Integrated.1-1") |
| `mac_address` | TEXT | MAC in Uppercase AA:BB:CC:DD:EE:FF |
| `ip_address` | TEXT | Zugewiesene IP (wenn vom BMC bekannt) |
| `link_status` | TEXT | Up, Down, Unknown |
| `link_speed_mbps` | INTEGER | Linkgeschwindigkeit in Mbit/s |
| `model` | TEXT | NIC-Bezeichnung |

**Indizes für schnelle Suche:**

```sql
CREATE INDEX idx_memory_search  ON memory(serial_number, manufacturer, part_number);
CREATE INDEX idx_storage_search ON storage(serial_number, manufacturer, model);
CREATE INDEX idx_network_mac    ON networks(mac_address);
CREATE INDEX idx_server_search  ON servers(chassis_serial, hostname);
```

---

## Architektur

```
bmcinv/
├── main.go                         # Entry Point – delegiert an cmd.Execute()
├── Makefile                        # build, install, test, install-completion
├── go.mod / go.sum
│
├── .github/
│   └── workflows/
│       └── ci.yml                  # CI: build, vet, test -race, golangci-lint
│
├── cmd/                            # CLI-Befehle (Cobra-Framework)
│   ├── root.go                     # PersistentPreRunE: Config + DB initialisieren
│   ├── scan.go                     # bmcinv scan – Worker-Pool starten
│   ├── find.go                     # bmcinv find – Datenbanksuche
│   ├── export.go                   # bmcinv export – CSV/JSON-Export
│   └── status.go                   # bmcinv status – Übersicht
│
├── internal/
│   ├── config/
│   │   ├── config.go               # Viper-Config, Env-Var-Bindings, Credential-Lookup
│   │   └── config_test.go
│   │
│   ├── database/
│   │   └── database.go             # GORM/SQLite Singleton, Upsert, Transaktionen
│   │
│   ├── models/
│   │   └── models.go               # Server, Memory, Storage, Network, SearchResult
│   │
│   ├── scanner/
│   │   ├── scanner.go              # Worker-Pool, CIDR-Expansion, scanHost-Workflow
│   │   ├── redfish.go              # HTTP-Client, Redfish-JSON-Typen, API-Abfragen
│   │   └── scanner_test.go
│   │
│   └── finder/
│       ├── finder.go               # Parallele Multi-Tabellen-Suche, MAC-Normalisierung
│       └── finder_test.go
│
└── man/
    └── bmcinv.1                    # Man Page
```

**Abhängigkeiten:**

| Paket | Zweck |
|-------|-------|
| `github.com/spf13/cobra` | CLI-Framework mit Subcommands |
| `github.com/spf13/viper` | Config-Verwaltung (YAML + Env-Vars) |
| `gorm.io/gorm` + `gorm.io/driver/sqlite` | ORM + SQLite-Treiber |
| Stdlib: `net/http`, `crypto/tls`, `encoding/json` | Redfish HTTP-Client |

---

## Troubleshooting

### "no Redfish endpoint reachable"

Der BMC antwortet nicht auf `https://<ip>/redfish/v1/`. Mögliche Ursachen:

```bash
# 1. BMC erreichbar?
ping <ip>

# 2. HTTPS-Port offen?
curl -k -v https://<ip>/redfish/v1/ 2>&1 | head -20

# 3. Firewall prüfen – BMCs sind oft nur in Management-VLANs erreichbar
# 4. Timeout erhöhen
bmcinv scan <ip> --timeout 60
```

### "HTTP 401" (Unauthorized)

Credentials stimmen nicht:

```bash
# Env-Variablen gesetzt?
echo $BMCINV_IDRAC_PASSWORD

# Manuell mit curl testen
curl -k -u root:meinPasswort https://<ip>/redfish/v1/Systems/

# Config-Datei prüfen
cat ~/.bmcinv/config.yaml
```

### "credential error: no credentials configured for BMC type: unknown"

Vendor-Erkennung hat den BMC-Typ nicht bestimmt. Was antwortet der BMC?

```bash
curl -k -v https://<ip>/redfish/v1/ 2>&1 | grep -E "(Server:|HTTP/|Dell|HPE|iDRAC|iLO)"

# IPMI-Credentials als Fallback setzen (für unbekannte Vendor)
export BMCINV_IPMI_USERNAME="admin"
export BMCINV_IPMI_PASSWORD="meinPasswort"
```

### Scan ist sehr langsam

```bash
# Mehr Worker (Standard: 10)
bmcinv scan 10.0.0.0/24 -w 30

# Timeout für nicht-reagierende Hosts reduzieren
bmcinv scan 10.0.0.0/24 --timeout 10

# Hinweis: Mehr als ~50 Worker können BMC-Firmware destabilisieren
```

### SQLite-Datenbankfehler

```bash
# Rechte und Existenz prüfen
ls -la ~/.bmcinv/

# DB löschen und neu anlegen (Daten gehen verloren!)
rm ~/.bmcinv/inventory.db
bmcinv status   # legt DB automatisch neu an
```

---

## Entwicklung

### Voraussetzungen

- Go 1.23+
- gcc (für CGo → mattn/go-sqlite3)

```bash
# Ubuntu/Debian
sudo apt install golang gcc

# macOS
brew install go
```

### Build und Test

```bash
git clone https://github.com/Maxsander123/bmcinv.git
cd bmcinv

# Abhängigkeiten
go mod tidy

# Bauen
make build

# Tests ausführen (inkl. Race Detector)
make test

# oder direkt:
go test -v -race ./...

# Einzelnes Paket
go test -v ./internal/finder/
go test -v ./internal/config/
go test -v ./internal/scanner/
```

### Linting

```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
golangci-lint run ./...
```

### Neuen BMC-Vendor hinzufügen

1. `internal/config/config.go` – neue `BMCType`-Konstante:

```go
BMCTypeLENOVO BMCType = "xclarity"
```

2. Default-Credentials in `setDefaults()` eintragen.

3. `internal/scanner/redfish.go`, Funktion `detectVendorHTTP()` – Erkennungslogik erweitern.

4. Env-Var-Bindings in `bindEnvVars()` und `applyEnvPasswords()` ergänzen.

---

## Bekannte Einschränkungen

| Einschränkung | Details |
|--------------|---------|
| **Nur Redfish v1** | BMCs ohne Redfish-Support (sehr alte iDRAC 6, reine IPMI-only-BMCs) werden nicht gescannt |
| **Nur IPv4** | Keine IPv6-Adressen oder CIDR-Ranges |
| **Nur Basic Auth** | Session-Token-Authentifizierung (manche HPE iLO 5+) wird nicht unterstützt |
| **Lokale SQLite-DB** | Kein Multi-User-Betrieb; kein zentraler Server |
| **Kein req/s Rate-Limit** | Die Worker-Zahl begrenzt Parallelität, aber kein Requests-per-Second-Throttling |

---

## Links

- **Repository:** https://github.com/Maxsander123/bmcinv
- **Issues:** https://github.com/Maxsander123/bmcinv/issues
- **DMTF Redfish-Spezifikation:** https://www.dmtf.org/standards/redfish

## Lizenz

MIT
