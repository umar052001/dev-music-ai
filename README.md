# Dev Music AI 🎵

**A music app that finds, plays, and downloads music — and uses smart AI to do the boring parts for you.**

Search almost any song, play it right in your browser, or download it as a clean
MP3 file. You can paste a messy list of songs (titles, links, whatever) and the
AI will tidy it up, find every song, and grab them all for you — neatly filed
by artist and album.

---

## Why this exists

Downloading music from YouTube usually means:

1. Opening a sketchy website that's full of pop-ups.
2. Copy-pasting links one at a time.
3. Fixing file names that look like `video-3847392-360p-final-FINAL.mp3`.
4. Doing this 20 times for a playlist.

That's the old way. This app throws a little AI at those annoyances so you can
paste a whole messy list of songs, hit one button, and walk away. When it's
done, every song is in the right folder with a clean name.

---

## What it can do

### 🎧 Play music instantly
- Search millions of songs (powered by YouTube music search).
- Click a result to start playing it right away — no waiting for a download.
- Full player: **spacebar = pause/play**, next / previous, loops, shuffle,
  auto-plays the next song when one ends.
- Shows a little animated visualizer while music plays.

### 💾 Download music properly
- Save any song as a high-quality MP3 file.
- Files land in folders by **artist → album** (or just artist, your pick).
- Clean, normal file names — no `-official-video-HD` junk.
- Everything is saved on *your* machine. No weird uploads anywhere.

### 🧠 AI helpers (the smart part)
The app can talk to a few different AI models. You pick which one. It uses AI
for the boring, messy parts:

- **Title clean-up** → turns `"Song Name - Official Music Video HD"` into
  `"Song Name"` (with the right artist).
- **Smart suggestions** → the "For You" tab thinks about what you listen to and
  suggests new songs you might like.
- **Playlist ideas** → describe a mood in one line ("late night driving") and
  it builds a playlist for you.
- **Batch import** → paste a whole messy list of songs and it structures it
  into clean title / artist / link rows, finds anything that's missing a link,
  and downloads them all.

### 🗂️ Your stuff, saved
- A **library** of everything you've downloaded, browsable by artist.
- An **activity log** of the songs you look at and play.
- Your preferences and search history are stored in a local database — private,
  on your computer.

### 🎨 A clean, friendly look
- Flat, modern design with **rounded corners** and **pill-shaped tabs**.
- Big, readable text.
- Light, dark, midnight, and custom themes.
- No fake "glass" effects — just clean and simple.

---

## How it's built

This is a two-piece app that runs on your own computer:

| Piece | Language | Job |
|-------|----------|-----|
| **Backend** | **Go** | Finds music (via `yt-dlp`), streams it, downloads it, talks to AI, saves the database |
| **Frontend** | **React + Vite** | The web page you look at: search box, player, tabs, settings panels |

The backend serves the web page and does all the heavy lifting. The frontend
just makes it look nice and respond fast.

Here's the flow when you hit **"Download"**:

```
You press Download
      │
      ▼
Go backend ──► yt-dlp (the tool that actually fetches from YouTube)
      │              │
      │              ▼
      │         grabs the audio
      │              │
      │              ▼
      └──► saves it as a clean MP3
              │
              ▼
         sorted into Artist/Album folders
```

The whole thing talks over a simple local API (a list of web addresses that
start with `/api/...`). The frontend calls these; the backend answers.

---

## What's under the hood

**Backend** (`main.go` — most of the app logic)
- Handles every `/api/...` request: search, stream, download, library, activity,
  suggestions, and the batch importer.
- Runs background download jobs and reports progress while they work.

**AI layer** (`llm.go` — the part that talks to AI models)
- One shared way to ask **any** of these providers:
  - **Ollama** (local models on your own PC, plus Ollama's cloud models)
  - **OpenAI**
  - **Groq** (an extra-fast OpenAI-compatible service)
  - **Anthropic** (Claude)
  - **Google Gemini**
- If your main AI provider is down, it automatically falls back to a local
  Ollama model so the app still works.

**Config** (`config.json`, created on first run — see `config.example.json`)
- All your AI choices live here (which provider, which model, any API keys).
- You can edit it in a text editor **or** change it through the app's **✦ AI**
  settings panel — it saves for you.
- The app comes set up to use **cloud AI models first**, and only uses your local
  computer's model as a backup.
- There's a ready-to-copy example at `config.example.json` if you ever need to
  start fresh.

**Database** (`devmusic.db`)
- A local SQLite file holding your library, activity, and search history.
- Entirely on your machine. Nothing is sent to any cloud database.

**Frontend** (`frontend/src/`)
- React components for every screen: search, player bar, library, activity,
  batch import, AI settings, and the theme picker.

---

## Setup & running it (step by step)

> You'll need a computer with **Go**, **Node.js**, and **yt-dlp** installed.
> If you want the AI features, you'll also need an AI provider (Ollama is the
> easiest — it's free and runs on your own computer).

### 1. Install the tools

- **Go** → https://go.dev/dl
- **Node.js** → https://nodejs.org
- **yt-dlp** → https://github.com/yt-dlp/yt-dlp (this is what actually grabs the
  audio). On most systems it installs with `pip install yt-dlp` or a package
  manager.

### 2. Download this project

```bash
git clone https://github.com/umar052001/dev-music-ai.git
cd dev-music-ai
```

### 3. Build the backend

```bash
# The database driver needs CGO enabled, so build with this flag:
CGO_ENABLED=1 go build -o music-server .
```

### 4. Install the frontend

```bash
cd frontend
npm install
npm run build
cd ..
```

### 5. (Optional but recommended) Start an AI provider

Pick whichever suits you. The easiest free option is **Ollama**:

```bash
# install Ollama from https://ollama.com, then:
ollama run gemma3:latest   # a free local model
```

The app's default config points at your local Ollama at `http://localhost:11434`.
If you'd rather use a cloud provider (OpenAI, Groq, Claude, Gemini), open the
app, hit the **✦ AI** button, and paste your API key and model — the settings
panel saves it to `config.json`.

### 6. Run it

```bash
# from the project folder
./music-server
```

Then open your browser to **http://localhost:8000**.

> **Tip:** the app listens on port **8000**. If you ever want to change it,
> edit the `addr` line near the top of `main.go` and rebuild.

### 7. Search, play, download

- Type a song in the search box.
- Hit **▶ Play** to hear it instantly, or **⬇ Download** to save an MP3.
- Paste a big messy list on the **Batch Import** tab and let the AI sort and
  grab them all.

---

## The API (for people who like details)

The browser talks to the backend through these addresses. Each one does one job:

| Endpoint | What it does |
|----------|--------------|
| `/api/search` | Search YouTube for a song |
| `/api/stream` | Stream audio for instant playback |
| `/api/download` | Download a song as an MP3 |
| `/api/library` | List everything you've downloaded |
| `/api/all-songs` | List every song you've ever looked at |
| `/api/file/...` | Serve a downloaded audio file |
| `/api/activity` | Your playback/search activity log |
| `/api/suggestions` | AI-powered "For You" suggestions |
| `/api/playlist-suggest` | AI builds a playlist from a mood |
| `/api/clean-title` | AI cleans up a messy song title |
| `/api/batch/parse` | AI structures a pasted list of songs |
| `/api/batch/run` | Starts the download job for a list |
| `/api/batch/status` | Reports download progress |
| `/api/llm/status` | Is the AI provider reachable? |
| `/api/llm/config` | Read the current AI settings |
| `/api/llm/config-set` | Save new AI settings |

---

## Project layout

```
dev-music-ai/
├── main.go              # The whole Go backend (server + all endpoints)
├── llm.go               # Talks to AI providers (Ollama, OpenAI, Groq, Claude, Gemini)
├── go.mod / go.sum      # Go dependency files
├── config.json          # Your AI settings (provider, models, keys) — created on first run
├── frontend/
│   ├── package.json     # Frontend dependencies (React, Vite, GSAP)
│   └── src/
│       ├── App.jsx      # The main page + tab layout
│       ├── components/  # Search, PlayerBar, Library, BatchImport, LLMSettings, ...
│       └── context/     # Player state + theme state
├── downloads/           # Where downloaded music lands (created on first use)
└── devmusic.db          # Local database: library, activity, history
```

---

## Things worth knowing

- **Audio comes from YouTube** via the `yt-dlp` tool. Please only download music
  you have the right to, and follow the rules of whatever platform you're pulling
  from. This project is a handy tool, not a license to grab copyrighted stuff.
- **Cloud AI calls** can take a few seconds the first time (the model has to
  "wake up"). Give it a moment — the app now tells you it's working.
- **Your data stays local.** The music files, database, and preferences are all
  on your computer.
- The app was built to **work without an AI provider too** — search, play, and
  download all function fine. The AI tabs just hide if no provider is connected.

---

## Tech summary

- **Backend:** Go, `net/http`, SQLite (`mattn/go-sqlite3`), `yt-dlp`
- **Frontend:** React 19, Vite, GSAP (animations)
- **AI:** pluggable providers — Ollama (local + cloud), OpenAI, Groq, Claude, Gemini

Made by **umar052001**. Pull requests and ideas welcome.
