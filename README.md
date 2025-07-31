# SABnzbd Post-Processor für CrowdNFO

Ein automatisches Post-Processing-Skript für SABnzbd, das NFO-Dateien, MediaInfo-Daten und File Lists zur CrowdNFO API hochlädt.

## 🚀 Features

### Core Funktionalität
- 🎬 **Video-Releases**: Findet automatisch die größte Videodatei und erstellt MediaInfo
- 🎵 **Audio-Releases**: Erkennt Musik/Hörbücher und wählt exemplarisch Track 01 für MediaInfo
- 📄 **NFO-Upload**: Lädt NFO-Dateien unabhängig von Medien-Dateien hoch
- 📋 **File Lists**: Automatische Erstellung und Upload von kompletten Dateilisten
- 🏷️ **Kategorie-Mapping**: Zuordnung via in der Config definierten Mappings sowie Regexes als Fallback
- 📊 **Hash-Berechnung**: SHA256-Hashes mit konfigurierbaren Größenlimits
- 📁 **Archivierung**: Speichert alle hochgeladenen Dateien lokal in einem `archive` Unterordner (lässt sich in Zukunft auch deaktivieren)

### Staffelpack-Unterstützung
- 📺 **Automatische Erkennung**: Erkennt Staffelpacks über den Dateinamen und die Anzahl der Episoden
- ✂️ **Episoden-Splitting**: Jede Episode wird als separates Release verarbeitet
- 📂 **Flexible Strukturen**: Unterstützt sowohl Hauptverzeichnis- als auch Unterverzeichnis-Layouts
- 📅 **ISO-Datumsformat**: Support für `yyyy-mm-dd` Episoden-Formate
- 📄 **Intelligente File Lists**: Nur relevante Dateien pro Episode (falls nicht in separaten Ordnern)

### Erweiterte Features
- 🔄 **UmlautAdaptarr Integration**: Abfrage von originalem Releasenamen bei durch den UA umbenannten Releases
- ⚙️ **Post-Processing-Scripts**: Führe weitere Skripte nach dem CrowdNFO-Upload aus

## 📋 Voraussetzungen
- **SABnzbd**: Deaktivierung der Entschleierung von Dateinamen in Settings > Switches > Post processing
- **CrowdNFO API Key**: Registriere dich auf [CrowdNFO](https://crowdnfo.net) und generiere einen API-Key auf deinem [Profil](https://crowdnfo.net/profile/details).
- **MediaInfo**: Installiere MediaInfo-CLI auf deinem System (die GUI-Version funktioniert dafür nicht!):
  - **Ubuntu/Debian**: `sudo apt-get install mediainfo`
  - **macOS**: `brew install mediainfo`
  - **Windows**: Download von MediaInfo-CLI (Portable, x64) ins Verzeichnis vom CrowdClient erfolgt automatisch, wenn keine Installation gefunden wurde.
  - **Docker-Mod**: Wird automatisch installiert

## 📦 Installation & Einrichtung

### Download & Setup (manuell)
1. Lade die entsprechende Binärdatei für dein System herunter und kopiere sie in dein SABnzbd Skript-Verzeichnis
2. Mache sie ausführbar: `chmod +x crowdclient-sabnzbd-linux-amd64` (Linux/Mac)
3. CrowdClient in SABnzbd den gewünschten Kategorien zuordnen (ggf. muss vorher noch unter Settings > Folders das Skript-Verzeichnis festgelegt werden)
   - Es wird **nicht empfohlen**, den CrowdClient für ausnahmslos **alle Downloads** zu aktivieren, sondern nach Möglichkeit nur für die Kategorien, die CrowdNFO unterstützt.
   Zudem wollen wir Müll und Spam vermeiden. :)
4. Führe einmalig im Terminal aus: `./crowdclient-sabnzbd-xxx-xxx 0 0 0 0 0 0 0` (alternativ beliebige NZB mit SABnzbd laden)
5. Dies erstellt eine `crowdclient-config.json` mit Standardeinstellungen

### Docker-Mod
Falls du SABnzbd in Docker mit dem linuxserver.io Image nutzt, kannst du den CrowdClient und alle Abhängigkeiten ganz einfach über einen Docker-Mod installieren.

Füge dazu in den SABnzbd Docker-Argumenten die Umgebungsvariable `DOCKER_MODS=ghcr.io/pixelhunterx/docker-mods:sabnzbd-crowdclient` hinzu.
Falls du bereits andere Mods nutzt, kannst du diese auch kombinieren, z.B. `DOCKER_MODS=ghcr.io/pixelhunterx/docker-mods:sabnzbd-crowdclient|linuxserver/mods:dummy` (separiert durch `|`).

Außerdem solltest du die Umgebungsvariable `SCRIPT_DIR` definieren, z.B. `SCRIPT_DIR="/path/to/your/scripts"`, um den Ordner für die CrowdClient Binary und Config festzulegen.
Dafür solltest du dein SABnzbd Skript-Verzeichnis verwenden, in dem ggf. auch andere Post-Processing-Skripte liegen (muss ggf. noch in den SABnzbd Einstellungen definiert werden).

**Hinweis: Hier muss der Pfad aus dem Container verwendet werden, nicht der Host-Pfad.**
Falls nicht gesetzt, wird standardmäßig `/data/scripts` verwendet.

### Basis-Konfiguration
Bearbeite die `crowdclient-config.json`:

```json
{
  "api_key": "DEIN_CROWDNFO_API_KEY",
  "base_url": "https://crowdnfo.net/api/releases",
  "mediainfo_path": "",
  "max_hash_file_size": "",
  "category_mappings": {
    "Movies": ["movies", "movie", "radarr", "film"],
    "TV": ["tv", "television", "sonarr", "series", "shows", "serien"],
    "Games": ["games", "gaming", "pc-games"],
    "Software": ["software", "apps", "programs"],
    "Music": ["music", "audio", "mp3"],
    "Audiobooks": ["audiobooks", "hoerbuch", "abook"],
    "Books": ["books", "ebooks", "epub"],
    "Other": ["other", "misc"]
  },
  "post_processing": {
    "global": {
      "enabled": false,
      "command": "",
      "arguments": []
    },
    "categories": {}
  },
  "umlautadaptarr": {
    "enabled": false,
    "base_url": "http://localhost:5005"
  }
}
```
Damit das Skript funktioniert, musst du deinen CrowdNFO API-Key in der `crowdclient-config.json` eintragen. Diesen findest du in deinem [Profil](https://crowdnfo.net/profile/details).

## 🔧 Erweiterte Konfiguration

### UmlautAdaptarr Integration
Falls der UmlautAdaptarr verwendet wird, sollte unbedingt der UmlautAdaptarr in der crowdclient-config.json des CrowdClients aktiviert werden, da sonst die falschen (geänderten) Releasenamen verarbeitet werden.
Dazu `"enabled"` auf `true` setzen und die `base_url` auf den korrekten Host konfigurieren.
```json
{
  "umlautadaptarr": {
    "enabled": true,
    "base_url": "http://localhost:5005"
  }
}
```
⚠️ **Unabhängig von der Art der Installation (sowohl Docker als auch nativ) muss beim UmlautAdaptarr zwingend die Umgebungsvariable `SETTINGS__EnableChangedTitleCache=true` gesetzt werden, 
damit die umbenannten Releasenamen temporär gespeichert und über die API bereitgestellt werden können.**

#### Docker Nutzer:
Statt `localhost` entweder die IP von deinem Docker-Host oder die Bridge IP `172.17.0.1` nutzen.
Ebenfalls kann der Name vom Container, also z.B. `umlautadaptarr` verwendet werden, 
hierfür ist jedoch erforderlich, dass sich UmlautAdaptarr und SABnzbd im gleichen Docker Network befinden. 
Je nach Network Setup können die Adressen natürlich aber auch abweichen.

**Wichtig**: Das Port Mapping 5005:5005 muss in Docker zwingend (wieder) aktiviert werden, dies war bei der Verwendung von Prowlarr+Proxy optional.
Da auf dem Port jedoch eine wichtige API läuft, ist der Port für den CrowdClient erforderlich. Falls ein anderes Port Mapping verwendet wird, muss
der Port in der `base_url` natürlich angepasst werden.

**⚠️🛡️ Wenn der UmlautAdaptarr auf einem öffentlich erreichbaren Server (z.B. einem VPS oder Seedbox) läuft, sollte kein Port Mapping genutzt werden, da die API keine Authentifizierung hat
(und somit die API öffentlich erreichbar wäre). Anstattdessen am besten das gleiche Docker Netzwerk nutzen und `http://umlautadaptarr:5005` (ggf. durch anderen Container-Namen ersetzen)
als `base_url` verwenden. Idealerweise nur `127.0.0.1:5005:5005` als Mapping nutzen, falls dies erforderlich ist und SABnzbd/\*arrs Host Networking nutzen.**


### Hash-Limits
Anpassung der Maximalgröße von Dateien für die SHA256-Berechnung:

```json
{
  "max_hash_file_size": "5GB"     // Limit auf 5GB
  "max_hash_file_size": "800MB"   // Limit auf 800MB  
  "max_hash_file_size": "0"       // Deaktiviert
  "max_hash_file_size": ""        // Kein Limit
}
```
Standardmäßig ist kein Limit eingestellt, je nach Leistung des Systems kann es sich jedoch empfehlen, für größere Dateien ein Limit einzustellen,
um die Last auf CPU und Datenträger zu reduzieren. Beispiele sind oben angegeben (Angabe ist in GB und MB möglich), zum gänzlichen Deaktivieren
der Hash-Berechnung muss der Wert auf "0" gesetzt werden.

### Post-Processing-Scripts
Führe zusätzliche Scripts nach CrowdNFO aus:

```json
{
  "post_processing": {
    "global": {
      "enabled": true,
      "command": "/path/to/script.sh",
      "arguments": ["--custom", "arg"]
    },
    "movies": {
      "enabled": true,
      "command": "/path/to/movie-script.sh",
      "arguments": []
    }
  }
}
```
Da der CrowdClient ggf. bereits vorhandene Post-Processing-Skripte in SABnzbd ersetzt, können in der Config entweder global oder je Kategorie 
Post-Processing-Skripte definiert werden. Diese werden nach der CrowdNFO Verarbeitung ausgeführt. Es werden alle Parameter von SABnzbd an das Script übergeben, sowie auch die Umgebungsvariablen.

Bei Docker bitte das korrekte Pfad-Mapping beachten (nicht die Pfade vom Host verwenden).

## 🛠️ Troubleshooting

### Häufige Probleme

**1. MediaInfo nicht gefunden**
- Stelle sicher, dass du MediaInfo-CLI installiert hast und der Pfad in der `crowdclient-config.json` korrekt gesetzt ist, insofern es sich nicht um das Standard-Installationsverzeichnis handelt.
Alternativ muss MediaInfo im PATH vorhanden sein oder im gleichen Verzeichnis wie der CrowdClient liegen.

**2. UmlautAdaptarr check failed**
- Wenn die Fehlermeldung `❌ UmlautAdaptarr check failed: UmlautAdaptarr API error (status 501): Set SETTINGS__EnableChangedTitleCache to true to use this endpoint.` auftritt,
fehlt die Umgebungsvariable `SETTINGS__EnableChangedTitleCache=true` in deiner UmlautAdaptarr-Installation. Du musst diese Variable setzen, damit die API korrekt funktioniert.

- Wenn die Fehlermeldung `❌ Umlautadaptarr check failed: failed to connect to Umlautadaptarr [...] connection refused` auftritt,
ist der UmlautAdaptarr nicht erreichbar. Überprüfe die URL und den Port in der `crowdclient-config.json` und stelle sicher, dass der Dienst läuft. Beachte auch die oben genannten Hinweise für Docker.


**3. NFO/MediaInfo/File List Upload failed**
- Falls die folgende Fehlermeldung `❌ <type> upload failed:{"message":"You have already submitted a file of this type to this release for this alias.","errorCode":"","details":null}`
auftritt, bedeutet dies, dass bereits eine NFO oder MediaInfo-Datei für dieses Release hochgeladen wurde. CrowdNFO erlaubt nur einen Upload pro Dateityp pro Release.

**4. API-Schlüssel Fehler**
- Überprüfe den CrowdNFO API-Key in der Config
- Stelle sicher, dass der Key aktiv ist

**5. Das Post Processing dauert bei großen Dateien sehr lang**
- Setze ein `max_hash_file_size` Limit, um die SHA256-Berechnung für große Dateien zu deaktivieren oder zu begrenzen.
  - Beispiel: `"max_hash_file_size": "10GB"` für ein Limit von 10GB
  - Oder deaktiviere mit `"max_hash_file_size": "0"` (keine Hash-Berechnung)

### Logs analysieren
- ✅ = Erfolgreich
- ❌ = Fehler
- ⚠️ = Warnung
- ⏭️ = Übersprungen

## 📝 Changelog

### Aktuelle Version
- ✨ **File Lists**: Automatische Erstellung und Upload von Dateilisten
- ✨ **Umlautadaptarr**: Integration für bessere Sonarr/Radarr-Kompatibilität
- ✨ **ISO-Datumsformat**: Support für `yyyy-mm-dd` Episoden-Format
- ✨ **Fallback-Erkennung**: Staffelpacks mit ≥3 Episoden automatisch erkennen
- ✨ **Docker-Support**: Intelligente Container-Erkennung und Netzwerk-Hilfe
- ✨ **Verbesserte File Lists**: Episode-spezifische Dateizuordnung
- 🔧 **Category Mapping**: Umgekehrtes Format (CrowdNFO → SABnzbd)
- 🔧 **Hash-Limits**: Konfigurierbare SHA256-Berechnung
- 🐛 **NFO-Zuordnung**: Korrekte Zuordnung für ISO-Format (kleinstes Datum)

## 🤝 Support

Bei Problemen oder Feature-Requests erstelle ein Issue im Repository oder einfach im #crowdnfo Channel bei Discord schreiben. :)
