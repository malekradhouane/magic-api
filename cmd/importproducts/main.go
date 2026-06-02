// Command importproducts charge un fichier CSV d'export "Rayon"
// (ex: E26-HOMME.csv) et insère son contenu en tant que produits.
//
// Usage :
//
//	go run ./cmd/importproducts --file E26-HOMME.csv
//	./bin/importproducts -f E26-HOMME.csv
//
// La connexion PostgreSQL est résolue comme pour le serveur : le port provient
// de config/magic.yaml, et l'hôte / login / mot de passe / base proviennent des
// variables d'environnement DB_HOST, DB_LOGIN, DB_PASSWORD, DB_NAME (.env).
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"

	"github.com/malekradhouane/magic/magic/configmanager"
	"github.com/malekradhouane/magic/service"
	"github.com/malekradhouane/magic/store"
	"github.com/malekradhouane/magic/store/postgres"
)

func main() {
	var (
		filePath string
		gender   string
	)
	flag.StringVar(&filePath, "file", "E26-HOMME.csv", "chemin du fichier CSV à importer")
	flag.StringVar(&filePath, "f", "E26-HOMME.csv", "chemin du fichier CSV à importer (raccourci)")
	flag.StringVar(&gender, "gender", "homme", "genre des produits (informe seulement le log)")
	flag.Parse()

	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})

	if err := godotenv.Load(); err != nil {
		logger.Warn("aucun fichier .env trouvé, utilisation des variables d'environnement")
	}

	client, err := setupPostgres()
	if err != nil {
		logger.WithError(err).Fatal("connexion PostgreSQL impossible")
	}
	defer client.Close()

	if err := store.CreatePostgresStores(nil); err != nil {
		logger.WithError(err).Fatal("initialisation des stores impossible")
	}

	ctx := context.Background()

	categoryService := service.NewCategoryService(store.Categories(), logger)
	// On s'assure que la taxonomie par défaut existe (idempotent).
	if err := categoryService.SeedDefaults(ctx); err != nil {
		logger.WithError(err).Warn("seed des catégories par défaut en échec")
	}
	productService := service.NewProductService(store.Products(), logger)

	importer := service.NewProductImporter(productService, categoryService, logger)

	logger.WithFields(logrus.Fields{"file": filePath, "gender": gender}).Info("import en cours…")
	result, err := importer.ImportFromFile(ctx, filePath)
	if err != nil {
		logger.WithError(err).Fatal("import échoué")
	}

	out, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(out))

	logger.WithFields(logrus.Fields{
		"created":  result.ProductsCreated,
		"skipped":  result.ProductsSkipped,
		"failed":   result.ProductsFailed,
		"variants": result.VariantsCreated,
	}).Info("import terminé")

	if result.ProductsFailed > 0 {
		os.Exit(1)
	}
}

// setupPostgres reproduit la configuration de connexion du serveur : le port
// vient de la configuration, le reste des variables d'environnement.
func setupPostgres() (*postgres.Client, error) {
	configRootDir := os.Getenv("MAGIC_CONFIG_ROOT_DIR")
	env := os.Getenv("ENVIRONMENT")

	cman, err := configmanager.DefaultManagerWithKonf().
		WithConfigRoot(configRootDir).
		WithEnvironment(env).
		Build()
	if err != nil {
		return nil, fmt.Errorf("chargement de la configuration: %w", err)
	}

	dbConfig := cman.Magic().Storage.PostgreSQL

	client := postgres.NewClient(postgres.ConnParams{
		Host:         os.Getenv("DB_HOST"),
		Port:         dbConfig.Port,
		Database:     os.Getenv("DB_NAME"),
		UserName:     os.Getenv("DB_LOGIN"),
		UserPassword: os.Getenv("DB_PASSWORD"),
	})

	if err := client.Initialise(); err != nil {
		client.Close()
		return nil, fmt.Errorf("initialisation du client: %w", err)
	}
	if err := client.Ping(); err != nil {
		client.Close()
		return nil, fmt.Errorf("ping de la base: %w", err)
	}
	return client, nil
}
