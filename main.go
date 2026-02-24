package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-contrib/cors" // <-- NEW: Import the CORS package
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson" 
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// --- Struct Definitions (Schema) ---

type CompanyRating struct {
	CompanyID string  `json:"company_id" bson:"company_id"`
	AvgRating float64 `json:"avg_rating" bson:"avg_rating"`
}

type GlobalRatings struct {
	AvgRating float64 `json:"avg_rating" bson:"avg_rating"`
}

type GlobalMetrics struct {
	SuccessRate     float64 `json:"success_rate" bson:"success_rate"`
	AvgResponseTime int     `json:"avg_response_time" bson:"avg_response_time"`
}

// Vendor represents the document structure for the global_vectors collection
type Vendor struct {
	VendorID           string          `json:"vendor_id" bson:"vendor_id"`
	Name               string          `json:"name" bson:"name"`
	Category           string          `json:"category" bson:"category"`
	CompanyWiseRatings []CompanyRating `json:"company_wise_ratings" bson:"company_wise_ratings"`
	GlobalRatings      GlobalRatings   `json:"global_ratings" bson:"global_ratings"`
	GlobalMetrics      GlobalMetrics   `json:"global_metrics" bson:"global_metrics"`
	TrustScore         float64         `json:"trust_score" bson:"trust_score"`
}

// --- Global Variables ---
var collection *mongo.Collection

// --- Database Setup ---
func connectDB() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		log.Fatal("You must set your 'MONGO_URI' environmental variable.")
	}

	clientOptions := options.Client().ApplyURI(uri)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Fatal(err)
	}

	// Ping the database
	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatal("Could not connect to MongoDB:", err)
	}

	fmt.Println("Connected to MongoDB!")

	dbName := os.Getenv("DB_NAME")
	colName := os.Getenv("COLLECTION_NAME")
	collection = client.Database(dbName).Collection(colName)
}

// --- Route Handlers ---
func createVendor(c *gin.Context) {
	var vendor Vendor

	// 1. Bind JSON to struct
	if err := c.ShouldBindJSON(&vendor); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 2. Define the filter and options for Upsert
	filter := bson.M{"vendor_id": vendor.VendorID}
	opts := options.Replace().SetUpsert(true)

	// 3. Replace the document if it exists, otherwise insert it
	result, err := collection.ReplaceOne(ctx, filter, vendor, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save vendor"})
		return
	}

	// 4. Return appropriate response based on whether it was created or updated
	if result.UpsertedCount > 0 {
		c.JSON(http.StatusCreated, gin.H{
			"message":    "New vendor created successfully",
			"vendor_id":  vendor.VendorID,
		})
	} else {
		c.JSON(http.StatusOK, gin.H{
			"message":    "Existing vendor updated successfully",
			"vendor_id":  vendor.VendorID,
		})
	}
}

// --- Route Handler to get all vendors ---
func getAllVendors(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// collection.Find with an empty bson.M{} fetches all documents
	cursor, err := collection.Find(ctx, bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch vendors: " + err.Error()})
		return
	}
	defer cursor.Close(ctx) // Ensure the cursor is closed to prevent memory leaks

	var vendors []Vendor
	// cursor.All automatically decodes all documents into our slice
	if err = cursor.All(ctx, &vendors); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error decoding vendor data: " + err.Error()})
		return
	}

	// If the database is empty, return an empty array instead of a null value
	if vendors == nil {
		vendors = []Vendor{}
	}

	// Return the count and the data
	c.JSON(http.StatusOK, gin.H{
		"count": len(vendors),
		"data":  vendors,
	})
}

func healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "online",
		"time":   time.Now().Format(time.RFC3339),
	})
}

func main() {
	// Initialize Database
	connectDB()

	// Initialize Router
	r := gin.Default()

	// --- NEW: Add CORS Middleware ---
	// cors.Default() enables default CORS settings (allows all origins, all methods)
	r.Use(cors.Default())

	r.GET("/health", healthCheck)

	// Define Routes
	r.POST("/api/vendor", createVendor)
	r.GET("/api/vendors", getAllVendors) 

	// Run Server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	r.Run(":" + port)
}