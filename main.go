package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"time"

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

	// Os handlers abaixo estão implementados no seu arquivo handlers.go
	mux.HandleFunc("/health", a.healthHandler)
	mux.HandleFunc("/validate", a.validateKeyHandler)

	createHandler := http.HandlerFunc(a.createKeyHandler)
	mux.Handle("/keys", a.masterKeyAuthMiddleware(createHandler))

	return mux
}

func main() {
	// 1. Recupera as variáveis de ambiente obrigatoriamente (Configuradas nas Secrets do EKS)
	connStr := os.Getenv("DB_URL_AUTH")
	if connStr == "" {
		log.Fatal("Erro crítico: A variável de ambiente DB_URL_AUTH não foi definida.")
	}

	driver := os.Getenv("DB_DRIVER_AUTH")
	if driver == "" {
		driver = "postgres"
	}

	masterKey := os.Getenv("MASTER_KEY")
	if masterKey == "" {
		log.Fatal("Erro crítico: A variável de ambiente MASTER_KEY não foi definida.")
	}

	// 2. Prepara o componente de conexão
	db, err := sql.Open(driver, connStr)
	if err != nil {
		log.Fatalf("Erro crítico na configuração do driver de banco: %v", err)
	}
	defer db.Close()

	// 3. Loop de Retry (Backoff) para conectar e rodar as migrações no Amazon RDS
	// Evita que o pod caia se o banco RDS estiver iniciando ou sob carga
	maxRetries := 5
	for i := 1; i <= maxRetries; i++ {
		log.Printf("Tentando conectar ao banco e rodar migrações (Tentativa %d de %d)...", i, maxRetries)

		err = db.Ping()
		if err == nil {
			err = initDatabase(db)
			if err == nil {
				log.Println("Conexão estabelecida e tabelas validadas com sucesso!")
				break
			}
		}

		log.Printf("Falha na tentativa %d: %v", i, err)
		if i == maxRetries {
			log.Fatalf("Erro fatal: Não foi possível sincronizar com o banco após %d tentativas. Encerrando.", maxRetries)
		}

		time.Sleep(3 * time.Second)
	}

	app := &App{
		DB:        db,
		MasterKey: masterKey,
	}

	// 4. Inicia o servidor HTTP na porta especificada pelo desafio
	port := ":8001"
	log.Printf("Serviço de Autenticação (Go) pronto e rodando na porta %s...", port)
	log.Fatal(http.ListenAndServe(port, app.Routes()))
}
