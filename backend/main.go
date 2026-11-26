package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID int `json:"id"`
	Name string `json:"name"`
	Email string `json:"email"`
}

type Account struct {
	BalanceCents int64 `json:"balance_cents"`
}

type Transaction struct {
	ID int `json:"id"`
	Kind string `json:"kind"`
	AmountCents int64 `json:"amount_cents"`
	CreatedAt time.Time `json:"created_at"`
}

var db *sql.DB

func main() {
	var err error
	db, err = sql.Open("postgres", getenv("DATABASE_URL", "postgres://bank:bank@localhost:5432/bankdb?sslmode=disable"))
	if err != nil { log.Fatal(err) }
	if err = db.Ping(); err != nil { log.Fatal(err) }

	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", cors(health))
	mux.HandleFunc("/api/register", cors(register))
	mux.HandleFunc("/api/login", cors(login))
	mux.HandleFunc("/api/balance", cors(requireAuth(balance)))
	mux.HandleFunc("/api/deposit", cors(requireAuth(deposit)))
	mux.HandleFunc("/api/withdraw", cors(requireAuth(withdraw)))
	mux.HandleFunc("/api/transactions", cors(requireAuth(transactions)))

	port := getenv("PORT", "8080")
	log.Printf("API ouvindo em http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

func getenv(key, fallback string) string { if v := os.Getenv(key); v != "" { return v }; return fallback }

func cors(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions { w.WriteHeader(http.NoContent); return }
		next(w, r)
	}
}

func respond(w http.ResponseWriter, status int, v any) { w.Header().Set("Content-Type", "application/json"); w.WriteHeader(status); json.NewEncoder(w).Encode(v) }
func bad(w http.ResponseWriter, msg string) { respond(w, http.StatusBadRequest, map[string]string{"error": msg}) }

func health(w http.ResponseWriter, r *http.Request) { respond(w, 200, map[string]string{"status":"ok"}) }

func register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { bad(w, "método inválido"); return }
	var in struct{Name, Email, Password string}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil { bad(w, "dados inválidos"); return }
	in.Email = strings.TrimSpace(strings.ToLower(in.Email))
	if in.Name == "" || in.Email == "" || len(in.Password) < 6 { bad(w, "preencha nome, email e senha com 6 caracteres"); return }
	hash, _ := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	tx, err := db.Begin(); if err != nil { respond(w, 500, map[string]string{"error":"falha interna"}); return }
	var userID int
	if err := tx.QueryRow(`INSERT INTO users(name,email,password_hash) VALUES($1,$2,$3) RETURNING id`, in.Name, in.Email, string(hash)).Scan(&userID); err != nil { tx.Rollback(); bad(w, "email já cadastrado"); return }
	if _, err := tx.Exec(`INSERT INTO accounts(user_id,balance_cents) VALUES($1,0)`, userID); err != nil { tx.Rollback(); respond(w, 500, map[string]string{"error":"falha interna"}); return }
	tx.Commit()
	respond(w, 201, map[string]any{"token": strconv.Itoa(userID), "user": User{ID:userID,Name:in.Name,Email:in.Email}})
}

func login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { bad(w, "método inválido"); return }
	var in struct{Email, Password string}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil { bad(w, "dados inválidos"); return }
	var u User; var hash string
	err := db.QueryRow(`SELECT id,name,email,password_hash FROM users WHERE email=$1`, strings.ToLower(strings.TrimSpace(in.Email))).Scan(&u.ID,&u.Name,&u.Email,&hash)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Password)) != nil { respond(w, 401, map[string]string{"error":"credenciais inválidas"}); return }
	respond(w, 200, map[string]any{"token":strconv.Itoa(u.ID), "user":u})
}

func requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter,r *http.Request){
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token == "" { respond(w,401,map[string]string{"error":"não autorizado"}); return }
		id,err := strconv.Atoi(token); if err != nil { respond(w,401,map[string]string{"error":"não autorizado"}); return }
		r.Header.Set("X-User-ID",strconv.Itoa(id)); next(w,r)
	}
}

func currentUser(r *http.Request) int { id,_:=strconv.Atoi(r.Header.Get("X-User-ID")); return id }

func balance(w http.ResponseWriter,r *http.Request){
	if r.Method != http.MethodGet { bad(w,"método inválido"); return }
	var cents int64
	if err:=db.QueryRow(`SELECT balance_cents FROM accounts WHERE user_id=$1`,currentUser(r)).Scan(&cents); err!=nil { respond(w,500,map[string]string{"error":"falha interna"}); return }
	respond(w,200,Account{BalanceCents:cents})
}

func deposit(w http.ResponseWriter,r *http.Request){ mutate(w,r,"deposit") }
func withdraw(w http.ResponseWriter,r *http.Request){ mutate(w,r,"withdraw") }

func mutate(w http.ResponseWriter,r *http.Request,kind string){
	if r.Method != http.MethodPost { bad(w,"método inválido"); return }
	var in struct{AmountCents int64 `json:"amount_cents"`}
	if err:=json.NewDecoder(r.Body).Decode(&in); err!=nil || in.AmountCents<=0 { bad(w,"valor inválido"); return }
	uid:=currentUser(r); tx,err:=db.Begin(); if err!=nil { respond(w,500,map[string]string{"error":"falha interna"}); return }
	var cents int64
	if err:=tx.QueryRow(`SELECT balance_cents FROM accounts WHERE user_id=$1 FOR UPDATE`,uid).Scan(&cents); err!=nil { tx.Rollback(); respond(w,500,map[string]string{"error":"falha interna"}); return }
	if kind=="withdraw" && cents<in.AmountCents { tx.Rollback(); bad(w,"saldo insuficiente"); return }
	next:=cents+in.AmountCents; if kind=="withdraw" { next=cents-in.AmountCents }
	if _,err:=tx.Exec(`UPDATE accounts SET balance_cents=$1 WHERE user_id=$2`,next,uid); err!=nil { tx.Rollback(); respond(w,500,map[string]string{"error":"falha interna"}); return }
	if _,err:=tx.Exec(`INSERT INTO transactions(account_id,kind,amount_cents) SELECT id,$1,$2 FROM accounts WHERE user_id=$3`,kind,in.AmountCents,uid); err!=nil { tx.Rollback(); respond(w,500,map[string]string{"error":"falha interna"}); return }
	if err:=tx.Commit(); err!=nil { respond(w,500,map[string]string{"error":"falha interna"}); return }
	respond(w,200,Account{BalanceCents:next})
}

func transactions(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodGet { bad(w,"método inválido"); return }
	rows,err:=db.Query(`SELECT t.id,t.kind,t.amount_cents,t.created_at FROM transactions t JOIN accounts a ON a.id=t.account_id WHERE a.user_id=$1 ORDER BY t.created_at DESC LIMIT 50`,currentUser(r)); if err!=nil { respond(w,500,map[string]string{"error":"falha interna"}); return }
	defer rows.Close(); var out []Transaction
	for rows.Next(){ var t Transaction; if err:=rows.Scan(&t.ID,&t.Kind,&t.AmountCents,&t.CreatedAt); err==nil { out=append(out,t) } }
	if out==nil { out=[]Transaction{} }; respond(w,200,out)
}
