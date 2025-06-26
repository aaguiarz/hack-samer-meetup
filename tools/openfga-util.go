package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/openfga/go-sdk/client"
)

func main() {
	var (
		apiURL   = flag.String("url", "http://localhost:8080", "OpenFGA API URL")
		action   = flag.String("action", "help", "Action to perform: help, create-store, list-stores, create-model")
		storeID  = flag.String("store", "", "Store ID")
		storeName = flag.String("name", "mapping-engine-store", "Store name")
		modelFile = flag.String("model", "", "Path to authorization model file")
	)
	flag.Parse()

	configuration := &client.Configuration{
		ApiUrl: *apiURL,
	}
	
	fgaClient := client.NewSdkClient(configuration)
	ctx := context.Background()

	switch *action {
	case "help":
		printHelp()
	case "create-store":
		createStore(ctx, fgaClient, *storeName)
	case "list-stores":
		listStores(ctx, fgaClient)
	case "create-model":
		if *storeID == "" || *modelFile == "" {
			log.Fatal("store and model flags are required for create-model action")
		}
		createModel(ctx, fgaClient, *storeID, *modelFile)
	case "list-models":
		if *storeID == "" {
			log.Fatal("store flag is required for list-models action")
		}
		listModels(ctx, fgaClient, *storeID)
	default:
		fmt.Printf("Unknown action: %s\n", *action)
		printHelp()
	}
}

func printHelp() {
	fmt.Println("OpenFGA Utility Tool")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  go run tools/openfga-util.go [flags]")
	fmt.Println()
	fmt.Println("Actions:")
	fmt.Println("  help         - Show this help message")
	fmt.Println("  create-store - Create a new store")
	fmt.Println("  list-stores  - List all stores")
	fmt.Println("  create-model - Create an authorization model")
	fmt.Println("  list-models  - List models for a store")
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  -url         OpenFGA API URL (default: http://localhost:8080)")
	fmt.Println("  -action      Action to perform")
	fmt.Println("  -store       Store ID (required for model operations)")
	fmt.Println("  -name        Store name (default: mapping-engine-store)")
	fmt.Println("  -model       Path to authorization model JSON file")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  # Create a store")
	fmt.Println("  go run tools/openfga-util.go -action=create-store -name=my-store")
	fmt.Println()
	fmt.Println("  # List stores")
	fmt.Println("  go run tools/openfga-util.go -action=list-stores")
	fmt.Println()
	fmt.Println("  # Create a model")
	fmt.Println("  go run tools/openfga-util.go -action=create-model -store=<store-id> -model=model.json")
}

func createStore(ctx context.Context, fgaClient *client.OpenFgaClient, name string) {
	body := client.CreateStoreRequest{
		Name: name,
	}

	response, err := fgaClient.CreateStore(ctx).Body(body).Execute()
	if err != nil {
		log.Fatalf("Failed to create store: %v", err)
	}

	fmt.Printf("Store created successfully!\n")
	fmt.Printf("Store ID: %s\n", response.Id)
	fmt.Printf("Store Name: %s\n", response.Name)
	fmt.Printf("Created At: %s\n", response.CreatedAt.String())
}

func listStores(ctx context.Context, fgaClient *client.OpenFgaClient) {
	response, err := fgaClient.ListStores(ctx).Execute()
	if err != nil {
		log.Fatalf("Failed to list stores: %v", err)
	}

	if len(response.Stores) == 0 {
		fmt.Println("No stores found")
		return
	}

	fmt.Printf("Found %d store(s):\n\n", len(response.Stores))
	for _, store := range response.Stores {
		fmt.Printf("ID: %s\n", store.Id)
		fmt.Printf("Name: %s\n", store.Name)
		fmt.Printf("Created At: %s\n", store.CreatedAt.String())
		if store.UpdatedAt != nil {
			fmt.Printf("Updated At: %s\n", store.UpdatedAt.String())
		}
		fmt.Println("---")
	}
}

func createModel(ctx context.Context, fgaClient *client.OpenFgaClient, storeID, modelFile string) {
	// Read the model file
	modelData, err := os.ReadFile(modelFile)
	if err != nil {
		log.Fatalf("Failed to read model file: %v", err)
	}

	// Parse the model JSON
	var authModel map[string]interface{}
	if err := json.Unmarshal(modelData, &authModel); err != nil {
		log.Fatalf("Failed to parse model JSON: %v", err)
	}

	// Extract schema_version and type_definitions
	schemaVersion, ok := authModel["schema_version"].(string)
	if !ok {
		log.Fatal("Model file must contain schema_version field")
	}

	typeDefinitions, ok := authModel["type_definitions"].([]interface{})
	if !ok {
		log.Fatal("Model file must contain type_definitions field")
	}

	// Convert type definitions
	var typeDefs []client.TypeDefinition
	for _, td := range typeDefinitions {
		tdBytes, _ := json.Marshal(td)
		var typeDef client.TypeDefinition
		if err := json.Unmarshal(tdBytes, &typeDef); err != nil {
			log.Fatalf("Failed to parse type definition: %v", err)
		}
		typeDefs = append(typeDefs, typeDef)
	}

	body := client.WriteAuthorizationModelRequest{
		SchemaVersion:   schemaVersion,
		TypeDefinitions: typeDefs,
	}

	options := client.ClientWriteAuthorizationModelOptions{
		StoreId: &storeID,
	}

	response, err := fgaClient.WriteAuthorizationModel(ctx).Body(body).Options(options).Execute()
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	fmt.Printf("Authorization model created successfully!\n")
	fmt.Printf("Model ID: %s\n", response.AuthorizationModelId)
}

func listModels(ctx context.Context, fgaClient *client.OpenFgaClient, storeID string) {
	options := client.ClientReadAuthorizationModelsOptions{
		StoreId: &storeID,
	}

	response, err := fgaClient.ReadAuthorizationModels(ctx).Options(options).Execute()
	if err != nil {
		log.Fatalf("Failed to list models: %v", err)
	}

	if len(response.AuthorizationModels) == 0 {
		fmt.Println("No authorization models found")
		return
	}

	fmt.Printf("Found %d authorization model(s):\n\n", len(response.AuthorizationModels))
	for _, model := range response.AuthorizationModels {
		fmt.Printf("Model ID: %s\n", model.Id)
		fmt.Printf("Schema Version: %s\n", model.SchemaVersion)
		fmt.Printf("Type Definitions: %d\n", len(model.TypeDefinitions))
		fmt.Println("---")
	}
}
