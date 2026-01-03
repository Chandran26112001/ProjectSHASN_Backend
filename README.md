# ProjectSHASN Backend

A Go-based REST API backend for the SHASN project that serves ideological cards from a MongoDB database. The backend provides endpoints to retrieve card data from two different decks (Gemini and GPT questions).

## Overview

This backend application connects to a MongoDB server and exposes REST API endpoints to fetch ideological card data. Cards can be retrieved randomly or sequentially based on their ID.

## Features

- **Dual Deck Support**: Manage two separate question decks
  - Gemini Questions (`GeminiQuestions` collection)
  - GPT Questions (`GptQuestions` collection)
- **Random Card Retrieval**: Get a random card from either deck
- **Sequential Card Retrieval**: Get the next card based on the current card ID with wrap-around support
- **CORS Enabled**: Supports cross-origin requests for development
- **MongoDB Integration**: Stores and retrieves card data from MongoDB

## Technology Stack

- **Language**: Go 1.25.0
- **Framework**: Gin (Web framework)
- **Database**: MongoDB
- **Key Dependencies**:
  - `github.com/gin-gonic/gin` - HTTP web framework
  - `go.mongodb.org/mongo-driver` - MongoDB driver for Go

## Project Structure

```
ProjectSHASN_Backend/
├── main.go                    # Main application with API endpoints
├── go.mod                     # Go module definition
├── README.md                  # This file
└── data/
    ├── Gemini_Questions.json  # Sample Gemini questions data
    └── GPT_Questions.json     # Sample GPT questions data
```

## API Endpoints

### 1. Get Random Card
- **Endpoint**: `GET /random`
- **Query Parameters**:
  - `deck` (string): The deck to retrieve from (`gemini` or `gpt`)
- **Response**: A random card object from the specified deck with an added `deck` field
- **Example**:
  ```
  GET http://localhost:8080/random?deck=gemini
  ```

### 2. Get Next Card
- **Endpoint**: `GET /next`
- **Query Parameters**:
  - `deck` (string): The deck to retrieve from (`gemini` or `gpt`)
  - `current_id` (integer, required): The ID of the current card
- **Response**: The next card in sequence (by ID), or the first card if wrapping around
- **Example**:
  ```
  GET http://localhost:8080/next?deck=gemini&current_id=5
  ```

## Database Configuration

- **Database Name**: `Project_SHASN`
- **Collections**:
  - `GeminiQuestions` - Stores Gemini question cards
  - `GptQuestions` - Stores GPT question cards
- **MongoDB Connection**: Connects to `mongodb://localhost:27017`

## Setup & Installation

### Prerequisites
- Go 1.25.0 or higher
- MongoDB server running on `localhost:27017`

### Steps

1. Clone the repository
2. Install dependencies:
   ```bash
   go mod download
   ```
3. Start MongoDB server
4. Run the application:
   ```bash
   go run main.go
   ```
5. The server will start on `http://localhost:8080`

## Card Structure

Cards are flexible documents stored in MongoDB. Each card should have:
- `_id` (integer): Unique identifier for the card
- Other fields specific to the question content (varies by source)

The API adds a `deck` field to responses indicating which deck the card came from.

## CORS Support

The API includes CORS headers for development convenience, allowing requests from any origin with the following allowed methods:
- GET
- OPTIONS
