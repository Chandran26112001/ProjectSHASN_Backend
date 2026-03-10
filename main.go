package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	_ "github.com/marcboeker/go-duckdb"
)

var db *sql.DB

type Card struct {
	ID       int             `json:"_id"`
	Deck     string          `json:"deck"`
	Question string          `json:"question"`
	Yes      json.RawMessage `json:"yes"`
	No       json.RawMessage `json:"no"`
	Caution  string          `json:"caution"`
}

type GenerateReq struct {
	Topic     string `json:"topic" binding:"required"`
	Brutality int    `json:"brutality" binding:"required,min=1,max=3"`
	Deck      string `json:"deck" binding:"required"`
}

func main() {
	// Load .env if it exists
	godotenv.Load()

	// Connect to local DuckDB file
	var err error
	db, err = sql.Open("duckdb", "data/shasn_dev.db")
	if err != nil {
		log.Fatal("Failed to open DuckDB:", err)
	}
	defer db.Close()

	initDB()

	r := gin.Default()

	// CORS for development convenience
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	r.GET("/random", getRandomCard)
	r.GET("/next", getNextCard)
	r.POST("/generate", generateCard)

	log.Println("Starting server on :8080...")
	r.Run(":8080")
}

func initDB() {
	queries := []string{
		`CREATE SEQUENCE IF NOT EXISTS seq_card_id START 1`,
		`CREATE TABLE IF NOT EXISTS cards (
			id INTEGER DEFAULT nextval('seq_card_id') PRIMARY KEY,
			deck VARCHAR,
			question VARCHAR,
			yes JSON,
			no JSON,
			caution VARCHAR
		)`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			log.Fatalf("Failed to execute init query %s: %v", q, err)
		}
	}

	// Check if empty
	var count int
	err := db.QueryRow("SELECT count(*) FROM cards").Scan(&count)
	if err != nil {
		log.Fatal("Failed to count cards:", err)
	}

	if count == 0 {
		log.Println("Database is empty. Hydrating from JSON files...")
		
		// Load Gemini
		_, err = db.Exec(`
			INSERT INTO cards (deck, question, yes, no, caution) 
			SELECT 'gemini', question, to_json(yes), to_json(no), caution 
			FROM read_json_auto('data/Gemini_Questions.json')
		`)
		if err != nil {
			log.Println("Warning: Could not load Gemini_Questions.json:", err)
		}

		// Load GPT
		_, err = db.Exec(`
			INSERT INTO cards (deck, question, yes, no, caution) 
			SELECT 'gpt', question, to_json(yes), to_json(no), caution 
			FROM read_json_auto('data/GPT_Questions.json')
		`)
		if err != nil {
			log.Println("Warning: Could not load GPT_Questions.json:", err)
		}
		
		// Attempt to load previously generated cards
		if _, err := os.Stat("data/Generated_Questions.json"); err == nil {
			_, err = db.Exec(`
				INSERT INTO cards (deck, question, yes, no, caution) 
				SELECT deck, question, to_json(yes), to_json(no), caution 
				FROM read_json_auto('data/Generated_Questions.json')
			`)
			if err != nil {
				log.Println("Warning: Could not load Generated_Questions.json:", err)
			}
		}

		log.Println("Hydration complete.")
	} else {
		log.Printf("DuckDB already contains %d cards. Skipping hydration.\n", count)
	}
}

func getRandomCard(c *gin.Context) {
	deck := c.Query("deck")
	if deck != "gpt" && deck != "gemini" {
		deck = "gemini" // default fallback
	}

	var card Card
	var yesStr, noStr string

	query := `SELECT id, deck, question, yes::VARCHAR, no::VARCHAR, caution FROM cards WHERE deck = ? ORDER BY RANDOM() LIMIT 1`
	err := db.QueryRow(query, deck).Scan(&card.ID, &card.Deck, &card.Question, &yesStr, &noStr, &card.Caution)
	
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "No cards found for this deck"})
		return
	} else if err != nil {
		log.Println("DB Query Error:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch card"})
		return
	}

	card.Yes = json.RawMessage(yesStr)
	card.No = json.RawMessage(noStr)

	c.JSON(http.StatusOK, card)
}

func getNextCard(c *gin.Context) {
	deck := c.Query("deck")
	if deck != "gpt" && deck != "gemini" {
		deck = "gemini"
	}
	currentIDStr := c.Query("current_id")
	
	if currentIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "current_id is required"})
		return
	}
	currentID, err := strconv.Atoi(currentIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid current_id format"})
		return
	}

	var card Card
	var yesStr, noStr string

	// Try to get next id
	query := `SELECT id, deck, question, yes::VARCHAR, no::VARCHAR, caution FROM cards WHERE deck = ? AND id > ? ORDER BY id ASC LIMIT 1`
	err = db.QueryRow(query, deck, currentID).Scan(&card.ID, &card.Deck, &card.Question, &yesStr, &noStr, &card.Caution)
	
	if err == sql.ErrNoRows {
		// Wrap around
		query = `SELECT id, deck, question, yes::VARCHAR, no::VARCHAR, caution FROM cards WHERE deck = ? ORDER BY id ASC LIMIT 1`
		err = db.QueryRow(query, deck).Scan(&card.ID, &card.Deck, &card.Question, &yesStr, &noStr, &card.Caution)
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "No cards found in deck"})
			return
		}
	}
	
	if err != nil {
		log.Println("DB Query Error:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch card"})
		return
	}

	card.Yes = json.RawMessage(yesStr)
	card.No = json.RawMessage(noStr)

	c.JSON(http.StatusOK, card)
}

func generateCard(c *gin.Context) {
	var req GenerateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	apiKey := os.Getenv("LLM_API_KEY")
	if apiKey == "" || apiKey == "dummy" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Missing LLM_API_KEY in .env"})
		return
	}

	// Prepare brutality prompt instructions
	var brutalityDesc string
	switch req.Brutality {
	case 1:
		brutalityDesc = "Make the question thought-provoking but mild. A civil policy debate."
	case 2:
		brutalityDesc = "Make the question edgy, provocative, and highly polarizing. High stakes."
	case 3:
		brutalityDesc = "Make the question absolutely ruthless, ethically agonizing, and highly divisive."
	}

	systemPrompt := fmt.Sprintf(`You are generating a dilemma card for a political strategy game.
Topic: %s
Brutality Level: %d
Instruction: %s

You must output ONLY raw JSON exactly matching the following schema. Do NOT include markdown blocks like \x60\x60\x60json.

Schema:
{
  "question": "The dilemma question here?",
  "yes": {
    "statement": "The affirmative argument statement.",
    "persona": "THE CAPITALIST"
  },
  "no": {
    "statement": "The negative argument statement.",
    "persona": "THE IDEALIST"
  },
  "caution": "none" // or "single" or "double"
}

Allowed Personas: THE CAPITALIST, THE SUPREMO, THE IDEALIST, THE SHOWSTOPPER.`, req.Topic, req.Brutality, brutalityDesc)

	// Build xAI Grok request
	payload := map[string]interface{}{
		"model": "grok-4-1-fast-non-reasoning",
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
		},
		"temperature": 0.8,
	}
	payloadBytes, _ := json.Marshal(payload)

	httpReq, _ := http.NewRequest("POST", "https://api.x.ai/v1/chat/completions", bytes.NewBuffer(payloadBytes))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed contacting Grok: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	// Read body once so we can log on any error
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read LLM response body"})
		return
	}
	log.Printf("Grok raw response (status %d): %s", resp.StatusCode, string(body))

	if resp.StatusCode != 200 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "LLM API returned error", "details": string(body)})
		return
	}

	var llmResp map[string]interface{}
	if err := json.Unmarshal(body, &llmResp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed decoding LLM response", "raw": string(body)})
		return
	}

	// Parse out content with safe type assertions
	choicesRaw, ok := llmResp["choices"]
	if !ok || choicesRaw == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "LLM response missing 'choices' field", "raw": llmResp})
		return
	}
	choices, ok := choicesRaw.([]interface{})
	if !ok || len(choices) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "LLM returned empty choices", "raw": llmResp})
		return
	}
	contentStr, ok := choices[0].(map[string]interface{})["message"].(map[string]interface{})["content"].(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "LLM response has unexpected shape", "raw": llmResp})
		return
	}
	contentStr = strings.TrimSpace(contentStr)
	contentStr = strings.TrimPrefix(contentStr, "```json")
	contentStr = strings.TrimPrefix(contentStr, "```")
	contentStr = strings.TrimSuffix(contentStr, "```")
	contentStr = strings.TrimSpace(contentStr)

	// Verify JSON structure
	var newCard map[string]interface{}
	if err := json.Unmarshal([]byte(contentStr), &newCard); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "LLM returned invalid JSON", "raw": contentStr})
		return
	}

	// Insert into DuckDB
	yesBytes, _ := json.Marshal(newCard["yes"])
	noBytes, _ := json.Marshal(newCard["no"])
	question := newCard["question"].(string)
	caution := newCard["caution"].(string)

	query := `INSERT INTO cards (deck, question, yes, no, caution) VALUES (?, ?, ?, ?, ?) RETURNING id`
	var newId int
	err = db.QueryRow(query, req.Deck, question, string(yesBytes), string(noBytes), caution).Scan(&newId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save card to DB: " + err.Error()})
		return
	}

	// Append to generated file for persistence across DB resets
	persistGeneratedCard(req.Deck, newCard)

	// Return to client
	returnCard := Card{
		ID:       newId,
		Deck:     req.Deck,
		Question: question,
		Yes:      yesBytes,
		No:       noBytes,
		Caution:  caution,
	}

	c.JSON(http.StatusOK, returnCard)
}

func persistGeneratedCard(deck string, card map[string]interface{}) {
	card["deck"] = deck // Store deck physically in the JSON so bootscript knows

	// We simply load the array, append, and save
	filePath := "data/Generated_Questions.json"
	
	var cards []map[string]interface{}
	data, err := os.ReadFile(filePath)
	if err == nil {
		json.Unmarshal(data, &cards)
	}

	cards = append(cards, card)
	
	newData, err := json.MarshalIndent(cards, "", "  ")
	if err == nil {
		os.WriteFile(filePath, newData, 0644)
	}
}
