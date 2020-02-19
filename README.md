## Building and running
### Windows:
From the CMD Prompt:
```
set GOOS=linux
go build -o main main.go
%USERPROFILE%\Go\bin\build-lambda-zip.exe -output main.zip main
```

### UNIX:
```
GOOS=linux GOARCH=amd64 go build -o main main.go && zip main.zip main && chmod 777 main.zip
```

## Running

### In AWS
Simply upload the zip file to lambda and set main as the function handler name, including an ACCESS_TOKEN env variable

### Locally
Pass no flags for a production run in Lambda

Pass the -menu flag to pull up the menu locally for adding the bot to new groups or removing it from old ones
