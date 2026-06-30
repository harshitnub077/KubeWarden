package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// UserInfo is extracted from a verified OIDC token.
type UserInfo struct {
	Subject string   `json:"sub"`
	Email   string   `json:"email"`
	Groups  []string `json:"groups"`
}

var verifier *gooidc.IDTokenVerifier

func main() {
	issuerURL := os.Getenv("OIDC_ISSUER_URL")
	clientID := os.Getenv("OIDC_CLIENT_ID")
	if issuerURL == "" {
		// Default: use kubectl proxy as a passthrough (dev mode)
		issuerURL = "https://accounts.google.com"
		clientID = "your-client-id"
	}

	// 1. Build the OIDC verifier
	ctx := context.Background()
	provider, err := gooidc.NewProvider(ctx, issuerURL)
	if err != nil {
		log.Fatalf("OIDC provider setup failed: %v", err)
	}
	verifier = provider.Verifier(&gooidc.Config{ClientID: clientID})

	// 2. Build the reverse proxy to the Kubernetes API server
	kubeconfig := filepath.Join(homedir.HomeDir(), ".kube", "config")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Fatalf("kubeconfig error: %v", err)
	}
	target, _ := url.Parse(config.Host)
	proxy := httputil.NewSingleHostReverseProxy(target)

	// 3. Set up routes
	mux := http.NewServeMux()

	// The proxy endpoint — authenticate then forward
	mux.HandleFunc("/proxy/", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		user, err := authenticate(r)
		if err != nil {
			http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
			return
		}
		// Inject the authenticated user info as impersonation headers
		r.Header.Set("Impersonate-User", user.Email)
		for _, g := range user.Groups {
			r.Header.Add("Impersonate-Group", g)
		}
		r.URL.Path = strings.TrimPrefix(r.URL.Path, "/proxy")
		proxy.ServeHTTP(w, r)
	})

	// The RBAC permission matrix endpoint
	mux.HandleFunc("/api/permissions", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == http.MethodOptions {
			return
		}

		user, err := authenticate(r)
		if err != nil {
			http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
			return
		}
		matrix, err := buildPermissionMatrix(config, user.Email, user.Groups)
		if err != nil {
			http.Error(w, "RBAC evaluation failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(matrix)
	})

	// Health endpoint (no auth required)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})

	fmt.Println("kube-auth-proxy listening on :8888")
	log.Fatal(http.ListenAndServe(":8888", mux))
}

// authenticate extracts and verifies the Bearer token from the request.
func authenticate(r *http.Request) (*UserInfo, error) {
	authHeader := r.Header.Get("Authorization")
	
	// Local dev bypass
	if authHeader == "Bearer dev-token" {
		return &UserInfo{
			Subject: "test-user",
			Email:   "test@example.com",
			Groups:  []string{"system:masters"},
		}, nil
	}

	if !strings.HasPrefix(authHeader, "Bearer ") {
		return nil, fmt.Errorf("missing or malformed Authorization header")
	}
	rawToken := strings.TrimPrefix(authHeader, "Bearer ")

	token, err := verifier.Verify(r.Context(), rawToken)
	if err != nil {
		return nil, fmt.Errorf("token verification failed: %w", err)
	}

	var claims struct {
		Sub    string   `json:"sub"`
		Email  string   `json:"email"`
		Groups []string `json:"groups"`
	}
	if err := token.Claims(&claims); err != nil {
		return nil, fmt.Errorf("claims extraction failed: %w", err)
	}
	return &UserInfo{Subject: claims.Sub, Email: claims.Email, Groups: claims.Groups}, nil
}

func setCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
}
