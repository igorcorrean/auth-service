package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	_ "github.com/lib/pq" // Driver do Postgres
)

// Definição da struct App
type App struct {
	DB        *sql.DB
	MasterKey string
}

// Inicializa a tabela caso ela não exista
func initDatabase(db *sql.DB) error {
	query := `
    CREATE TABLE IF NOT EXISTS api_keys (
        id SERIAL PRIMARY KEY, 
        key_hash CHAR(64) NOT NULL UNIQUE,
        name VARCHAR(100) NOT NULL,
        created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
        is_active BOOLEAN DEFAULT TRUE
    );
    CREATE INDEX IF NOT EXISTS idx_api_keys_hash ON api_keys(key_hash);
    `
	_, err := db.Exec(query)
	return err
}

// Configura o roteamento da aplicação
func (a *App) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", a.healthHandler)
	mux.HandleFunc("/validate", a.validateKeyHandler)

	createHandler := http.HandlerFunc(a.createKeyHandler)
	mux.Handle("/keys", a.masterKeyAuthMiddleware(createHandler))

	return mux
}

func main() {
	// 1. Pega a string de conexão das variáveis de ambiente
	connStr := os.Getenv("DATABASE_URL_AUTH")
	driver := os.Getenv("DB_DRIVER_AUTH")
	if connStr == "" {
		connStr = "postgres://postgres:senha_secreta_auth@db-auth:5432/auth_db?sslmode=disable"
	}
	if driver == "" {
		driver = "postgres"
	}
	// 2. Conecta ao banco de dados
	db, err := sql.Open(driver, connStr)
	if err != nil {
		log.Fatalf("Erro ao conectar no banco: %v", err)
	}
	defer db.Close()

	// 3. Garante que a tabela api_keys existe
	err = initDatabase(db)
	if err != nil {
		log.Fatalf("Erro ao rodar migração automática: %v", err)
	}

	// 4. Pega a Master Key do ambiente
	masterKey := os.Getenv("MASTER_KEY")
	if masterKey == "" {
		masterKey = "chave_temporaria_local"
	}

	app := &App{
		DB:        db,
		MasterKey: masterKey,
	}

	// 5. Configura e define a porta (corrigindo o erro de 'undefined: port')
	port := ":8001"
	log.Printf("Serviço de Autenticação (Go) rodando na porta %s...", port)
	log.Fatal(http.ListenAndServe(port, app.Routes()))
}
