package main

import (
	"context"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var client *mongo.Client

// Card represents the structure of the card in the database
// Using map[string]interface{} for flexibility since we just need to pass it through
// and add one field.
type Card map[string]interface{}

const (
	CollectionGemini = "GeminiQuestions"
	CollectionGpt    = "GptQuestions"
)

func main() {
	// Connect to MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var err error
	client, err = mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		log.Fatal(err)
	}

	// Verify connection
	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatal("Could not connect to MongoDB:", err)
	}
	log.Println("Connected to MongoDB")

	r := gin.Default()

	// CORS for development convenience
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	r.GET("/random", getRandomCard)
	r.GET("/next", getNextCard)

	r.Run(":8080")
}

func getCollectionName(deck string) string {
	if deck == "gpt" {
		return CollectionGpt
	}
	return CollectionGemini
}

// getRandomCard gets a random document from the specified deck
func getRandomCard(c *gin.Context) {
	deck := c.Query("deck")
	collectionName := getCollectionName(deck)
	collection := client.Database("Project_SHASN").Collection(collectionName) // Using Project_SHASN database

	// Get count to pick a random index
	count, err := collection.CountDocuments(context.Background(), bson.D{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count documents"})
		return
	}

	if count == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "No cards found in deck"})
		return
	}

	// Determine random skip
	rand.Seed(time.Now().UnixNano())
	skip := rand.Int63n(count)

	var card Card
	opts := options.FindOne().SetSkip(skip)
	err = collection.FindOne(context.Background(), bson.D{}, opts).Decode(&card)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch card"})
		return
	}

	// Add the deck field
	card["deck"] = deck

	// Ensure deck is strictly gemini or gpt for the response
	if deck == "gpt" {
		card["deck"] = "gpt"
	} else {
		card["deck"] = "gemini"
	}

	c.JSON(http.StatusOK, card)
}

// getNextCard gets the next card in sequence based on current_id
func getNextCard(c *gin.Context) {
	deck := c.Query("deck")
	currentIDStr := c.Query("current_id")

	if currentIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "current_id is required"})
		return
	}

	// Assuming IDs are integers as per previous context
	// If they are not, this logic will need to change.
	// We'll try to parse as int, if fails, we might need to handle string IDs.
	// Given the user prompt context "unique ID starting from 1", int is almost certain.
	currentID, err := strconv.Atoi(currentIDStr)
	if err != nil {
		// Fallback for non-integer IDs?
		// For now, let's assume they are integers.
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid current_id format, expected integer"})
		return
	}

	collectionName := getCollectionName(deck)
	collection := client.Database("Project_SHASN").Collection(collectionName)

	// Logic: Find one where _id > currentID, sorted by _id ascending.
	// If current is last, maybe wrap around? The prompt says "next sequence", implies linear.
	// Be safe: if no next, return first (wrap around) or 404.
	// Let's implement wrap-around for endless play.

	filter := bson.M{"_id": bson.M{"$gt": currentID}}
	opts := options.FindOne().SetSort(bson.D{{Key: "_id", Value: 1}})

	var card Card
	err = collection.FindOne(context.Background(), filter, opts).Decode(&card)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			// Try wrapping around to the first document
			opts = options.FindOne().SetSort(bson.D{{Key: "_id", Value: 1}})
			err = collection.FindOne(context.Background(), bson.D{}, opts).Decode(&card)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "No next card found"})
				return
			}
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch card"})
			return
		}
	}

	// Add the deck field
	if deck == "gpt" {
		card["deck"] = "gpt"
	} else {
		card["deck"] = "gemini"
	}

	c.JSON(http.StatusOK, card)
}
