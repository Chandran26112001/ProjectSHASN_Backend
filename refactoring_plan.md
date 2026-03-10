# Refactoring Plan and Feasibility Analysis

Here is a breakdown of your 5 suggestions, assessing their feasibility and providing an implementation strategy for the SHASN backend.

## 1. DuckDB vs. MongoDB and Static File Persistence
**Feasibility: Very High**
DuckDB is an excellent choice for a lightweight, analytical, or local-first application. Unlike MongoDB requiring a standalone server background process (`mongod`), DuckDB runs completely in-process within your Go application, using a local `.db` file. 

*   **Implementation:** We can use the `github.com/marcboeker/go-duckdb` driver (which acts like standard `database/sql` in Go).
*   **Static Storing (JSON):** DuckDB is heavily integrated with JSON. Not only can we store the output into static `.json` files, but DuckDB can also query JSON files directly with `SELECT * FROM read_json_auto('data/GPT_Questions.json')`. This completely removes the need for an external DB service.

## 2. LLM "Generate a Random Card" Feature (Topic & Brutality)
**Feasibility: High**
Generating cards dynamically via an LLM fits perfectly with the creative nature of the game.

*   **API Design:** 
    *   Endpoint: `POST /generate`
    *   Parameters: `topic` (string), `brutality` (int 1-3)
*   **LLM Integration:** Within Go, we make a REST call to the LLM's API (e.g., OpenAI or Gemini). We instruct the LLM (via a system prompt) to output *exactly* the JSON structure your cards follow (`question`, `yes`, `no`, `caution`). 
*   **Prompting Strategy for "Brutality":**
    *   1 = Mild/Thought-provoking.
    *   2 = Edgy/Provocative.
    *   3 = Ruthless/Highly divisive.
*   **Persistence:** Once the LLM returns the JSON, the Go backend immediately appends it to your static `Generated_Questions.json` (or inserts it into the DuckDB table which is synced to the file).

## 3. Bootscript for DuckDB (Hydrating from Static Cards)
**Feasibility: Very High**
Because DuckDB is spun up in-memory or from a file instantly when the Go API starts, a "bootscript" is just an initialization function in `main.go`.

*   **Implementation:**
    1. On `main()` start, connect to a local DuckDB file (e.g., `shasn.db`).
    2. Create the `Cards` table if it doesn't exist.
    3. Run a query: `INSERT INTO Cards SELECT * FROM read_json_auto('data/*.json')` to dynamically load all static `.json` files into the active DB on boot. Or, just query the JSON files directly when `GET /random` is called!

## 4. Using a Local `.env` for LLM Token Security
**Feasibility: Very High**
This is the standard and correct way to manage secrets.

*   **Implementation:** We will use a Go package like `github.com/joho/godotenv`.
*   You create a `.env` file locally containing `LLM_API_KEY=your_token_here`. (Ensure `.env` is added to `.gitignore`).
*   The Go backend reads this upon startup and uses it for the LLM HTTP requests. Anyone running it locally only needs to supply their own `.env` file or export the variable.

## 5. Deployment, Architecture, and Mobile Play Strategy
**Feasibility: Medium-High (Requires an architectural decision)**

You want to generate cards locally (to hide your API key) but *also* use the app on your mobile while hosting. You cannot expose a `localhost` Go app to your mobile phone over the internet natively without a tunnel, and you don't want to deploy your LLM key to a public server if others might use the API.

Here is the best strategy to handle this:

**Phase 1: Pre-game Preparation (Local Generation)**
*   You run the Go backend **locally** on your Mac.
*   It uses your local `.env` and LLM key.
*   You fire up a web UI (or just use curl/Postman) and hit the `/generate` endpoint with various topics and brutalities.
*   The newly generated cards are appended to `data/Generated_Questions.json` on your Mac.

**Phase 2: Hosting for the Game (Mobile Access)**
*   **Option A (Local Network Only - Easiest):** If your Mac and Mobile are on the same Wi-Fi, you don't need to deploy it to the cloud. You start the Go server, find your Mac's local IP (e.g., `192.168.1.5`), and access `http://192.168.1.5:8080/random` from your phone's browser.
*   **Option B (Ngrok Tunnel - Easy internet access):** Run `ngrok http 8080` on your Mac. Ngrok gives you a public URL (e.g., `https://xyz.ngrok.io`) that you can open on your phone anywhere in the world, securely tunneling into your Mac's Go server.
*   **Option C (True Cloud Deployment):** 
    1. You commit your new `Generated_Questions.json` to GitHub.
    2. You deploy the Go API to a free service like **Render** or **Railway**. 
    3. The deployed version simply reads the static JSON files/DuckDB and serves `/random` and `/next`.
    4. *Crucially*, you **disable** the `/generate` endpoint in the deployed version (or protect it with a hardcoded password) so the public can't drain your LLM API credits.

### Recommended Path Forward
I suggest we tackle this in the following order:
1. Strip out MongoDB and integrate DuckDB to read from your static `/data` files.
2. Setup the `.env` handling for safety.
3. Build the `/generate` LLM integration route and standardise the Prompt engineering for "Brutality".
4. Handle the JSON saving/DuckDB persistence of those generated cards.

Would you like me to start rewriting `main.go` to replace Mongo with DuckDB as Step 1?
