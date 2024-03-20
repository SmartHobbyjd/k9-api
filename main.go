package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"

	pb "github.com/SmartHobbyjd/k9-api/contentpb"

	_ "github.com/mattn/go-sqlite3"
	"google.golang.org/grpc"
)

const (
	dbPath = "alphabyte.db"
	port   = ":50051" // Specify the port number here
)

func main() {
	// Open the SQLite database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Create tables if they don't exist
	if err := createTables(db); err != nil {
		log.Fatalf("failed to create tables: %v", err)
	}

	// Start gRPC server
	if err := startGRPCServer(db); err != nil {
		log.Fatalf("failed to start gRPC server: %v", err)
	}

	// Block main goroutine to keep the program running
	select {}
}

func createTables(db *sql.DB) error {
	_, err := db.Exec(`
        CREATE TABLE IF NOT EXISTS content (
            id INTEGER PRIMARY KEY,
            title TEXT,
            body TEXT,
            created_at INTEGER,
            updated_at INTEGER
        );

        CREATE TABLE IF NOT EXISTS images (
            id INTEGER PRIMARY KEY,
            content_id INTEGER,
            url TEXT,
            filename TEXT,
            type INTEGER
        );
    `)
	return err
}

func startGRPCServer(db *sql.DB) error {
	// Create listener on specified port
	lis, err := net.Listen("tcp", port)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}
	defer lis.Close()

	fmt.Printf("Server listening on port %s\n", port)

	// Create gRPC server
	grpcServer := grpc.NewServer()

	// Instantiate YourService with the database reference
	yourService := &YourService{DB: db}

	// Register YourService with the gRPC server
	pb.RegisterContentServiceServer(grpcServer, yourService)

	// Serve gRPC requests
	if err := grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("failed to serve: %v", err)
	}

	return nil
}

// YourService implements the ContentServiceServer interface
type YourService struct {
	pb.UnimplementedContentServiceServer
	DB *sql.DB
}

// CreateContent implements the CreateContent RPC method
func (s *YourService) CreateContent(ctx context.Context, in *pb.Content) (*pb.Content, error) {
	_, err := s.DB.Exec(`
        INSERT INTO content (id, title, body, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?)
    `, in.Id, in.Title, in.Body, in.CreatedAt, in.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create content: %v", err)
	}

	for _, img := range in.Images {
		_, err := s.DB.Exec(`
            INSERT INTO images (content_id, url, filename, type)
            VALUES (?, ?, ?, ?)
        `, in.Id, img.Url, img.Filename, img.Type)
		if err != nil {
			return nil, fmt.Errorf("failed to insert image: %v", err)
		}
	}

	return in, nil
}

// GetContent implements the GetContent RPC method
func (s *YourService) GetContent(ctx context.Context, in *pb.GetContentRequest) (*pb.Content, error) {
	// Query content by ID
	rows, err := s.DB.Query(`
        SELECT id, title, body, created_at, updated_at
        FROM content
        WHERE id = ?
    `, in.Id)
	if err != nil {
		return nil, fmt.Errorf("failed to query content: %v", err)
	}
	defer rows.Close()

	var content pb.Content
	for rows.Next() {
		err := rows.Scan(&content.Id, &content.Title, &content.Body, &content.CreatedAt, &content.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan content: %v", err)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to retrieve content: %v", err)
	}

	// Query images
	rows, err = s.DB.Query(`
        SELECT url, filename, type
        FROM images
        WHERE content_id = ?
    `, in.Id)
	if err != nil {
		return nil, fmt.Errorf("failed to query images: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var img pb.Image
		err := rows.Scan(&img.Url, &img.Filename, &img.Type)
		if err != nil {
			return nil, fmt.Errorf("failed to scan image: %v", err)
		}
		content.Images = append(content.Images, &img)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to retrieve images: %v", err)
	}

	return &content, nil
}

// UpdateContent implements the UpdateContent RPC method
func (s *YourService) UpdateContent(ctx context.Context, in *pb.Content) (*pb.Content, error) {
	// Update content in the database
	if _, err := updateContent(s.DB, in); err != nil {
		return nil, err
	}

	// Return the updated content
	return in, nil
}

// DeleteContent implements the DeleteContent RPC method
func (s *YourService) DeleteContent(ctx context.Context, in *pb.DeleteContentRequest) (*pb.Empty, error) {
	// Delete content from the database
	if err := deleteContent(s.DB, in.Id); err != nil {
		return nil, err
	}

	// Return an empty response
	return &pb.Empty{}, nil
}
