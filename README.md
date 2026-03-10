# bmcinv - BMC Inventory Tool

Ein Read-Only Hardware-Inventarisierungs-Tool für Rechenzentren, das via Redfish API (iDRAC, iLO, IPMI) Server abfragt und Hardware-Details in einer lokalen SQLite-Datenbank cached.

## Features

- 🔍 **Schnelle Suche** - Finde Server anhand von RAM-Seriennummern, MAC-Adressen, Disk-Serials
- 🔐 **Smart Credentials** - Verschiedene Passwörter für verschiedene BMC-Typen (Dell iDRAC, HPE iLO, IPMI)
- ⚡ **Paralleles Scanning** - Worker-Pool für schnelles Scannen großer IP-Bereiche
- 💾 **SQLite Cache** - Sekundenschnelle Suche ohne Netzwerkzugriff

## Installation

### Option 1: go install (empfohlen)
```bash
go install github.com/Maxsander123/bmcinv@latest
```

### Option 2: Von Source
```bash
git clone https://github.com/Maxsander123/bmcinv.git
cd bmcinv
make install   # Installiert nach /usr/local/bin (braucht sudo)
```

### Option 3: Nur für aktuellen User
```bash
make install-user  # Installiert nach ~/.local/bin
```

## Verwendung

```bash
# Status und Config anzeigen
bmcinv status

# Einzelnen Server scannen
bmcinv scan 192.168.1.100

# Ganzes Subnetz scannen
bmcinv scan 192.168.1.0/24

# Nach MAC-Adresse suchen
bmcinv find 00:1B:21:AB:CD:EF

# Nach RAM-Seriennummer suchen  
bmcinv find MEM12345678

# Alle Samsung-DIMMs finden
bmcinv find Samsung --type memory

# Alle Dell-Server finden
bmcinv find "Dell Inc."
```

## Konfiguration

Die Config-Datei liegt unter `~/.bmcinv/config.yaml`:

```yaml
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

scan:
  workers: 10
  timeout_secs: 30
```

## Architektur

```
bmcinv/
├── main.go                    # Entry Point
├── cmd/                       # CLI Commands (Cobra)
│   ├── root.go
│   ├── scan.go
│   ├── find.go
│   └── status.go
└── internal/
    ├── config/               # Viper Config + Smart Credentials
    ├── database/            # GORM/SQLite Layer
    ├── models/              # Server, Memory, Storage, Network
    ├── scanner/             # Worker-Pool + Vendor Detection
    └── finder/              # Parallele Multi-Table Suche
```

## Beispiel-Output

```
$ bmcinv find Samsung

=== Found 8 Result(s) ===

─── MEMORY (8) ───
SERVER IP      COMPONENT                          MATCHED        VALUE
192.168.1.10   Slot: DIMM_A1, 64GB Samsung DDR4   manufacturer   Samsung
192.168.1.10   Slot: DIMM_A2, 64GB Samsung DDR4   manufacturer   Samsung
192.168.1.15   Slot: DIMM_B1, 32GB Samsung DDR4   manufacturer   Samsung
```

## License

MIT
