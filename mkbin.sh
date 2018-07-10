echo "Building win..."
env GOARCH=amd64 GOOS=windows go build -o bin/party-line-win.exe 
echo "Building macos..."
env GOARCH=amd64 GOOS=darwin go build -o bin/party-line-macos
echo "Building linux..."
env GOARCH=amd64 GOOS=linux go build -o bin/party-line-linux