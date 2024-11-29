package main

import (
    "encoding/base64"
    "encoding/json"
    "encoding/xml"
    "fmt"
    "log"
    "net/http"
    "os"
    "sync"

    "github.com/gorilla/websocket"
    "github.com/joho/godotenv"
)

// Configuration constants
const (
    systemMessage = `You are Samantha. You are a helpful and bubbly AI assistant who loves to chat about 
        anything the user is interested in and is prepared to offer them facts. 
        Always stay positive, but work in a joke when appropriate.`
    voice = "alloy"
    openAIWSURL = "wss://api.openai.com/v1/realtime?model=gpt-4o-realtime-preview-2024-10-01"
)

var (
    openAIAPIKey string
    upgrader     = websocket.Upgrader{
        CheckOrigin: func(r *http.Request) bool {
            return true
        },
    }
    logEventTypes = []string{
        "response.content.done",
        "rate_limits.updated",
        "response.done",
        "input_audio_buffer.committed",
        "input_audio_buffer.speech_stopped",
        "input_audio_buffer.speech_started",
        "session.created",
    }
)

// TwiML structures
type VoiceResponse struct {
    XMLName xml.Name `xml:"Response"`
    Say     []Say    `xml:"Say,omitempty"`
    Pause   Pause    `xml:"Pause,omitempty"`
    Connect Connect  `xml:"Connect,omitempty"`
}

type Say struct {
    Text string `xml:",chardata"`
}

type Pause struct {
    Length int `xml:"length,attr"`
}

type Connect struct {
    Stream Stream `xml:"Stream"`
}

type Stream struct {
    URL string `xml:"url,attr"`
}

// Message structures
type TwilioMessage struct {
    Event string `json:"event"`
    Media struct {
        Payload string `json:"payload"`
    } `json:"media"`
    Start struct {
        StreamSid string `json:"streamSid"`
    } `json:"start"`
}

type SessionUpdate struct {
    Type    string `json:"type"`
    Session struct {
        TurnDetection struct {
            Type string `json:"type"`
        } `json:"turn_detection"`
        InputAudioFormat  string   `json:"input_audio_format"`
        OutputAudioFormat string   `json:"output_audio_format"`
        Voice            string   `json:"voice"`
        Instructions     string   `json:"instructions"`
        Modalities      []string `json:"modalities"`
        Temperature     float64  `json:"temperature"`
    } `json:"session"`
}

func init() {
    // Load .env file
    if err := godotenv.Load(); err != nil {
        log.Println("No .env file found")
    }

    openAIAPIKey = os.Getenv("OPENAI_API_KEY")
    if openAIAPIKey == "" {
        log.Fatal("Missing OpenAI API key")
    }
}

func main() {
    port := os.Getenv("PORT")
    if port == "" {
        port = "5050"
    }

    http.HandleFunc("/", handleIndex)
    http.HandleFunc("/incoming-call", handleIncomingCall)
    http.HandleFunc("/media-stream", handleMediaStream)

    log.Printf("Server starting on port %s", port)
    log.Fatal(http.ListenAndServe(":"+port, nil))
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
    json.NewEncoder(w).Encode(map[string]string{
        "message": "Twilio Media Stream Server is running!",
    })
}

func handleIncomingCall(w http.ResponseWriter, r *http.Request) {
    response := VoiceResponse{
        Say: []Say{
            {Text: "Please wait while we connect your call to the A. I. voice assistant, powered by Twilio and the Open-A.I. Realtime API"},
            {Text: "O.K. you can start talking!"},
        },
        Pause: Pause{Length: 1},
        Connect: Connect{
            Stream: Stream{
                URL: fmt.Sprintf("wss://%s/media-stream", r.Host),
            },
        },
    }

    w.Header().Set("Content-Type", "application/xml")
    xml.NewEncoder(w).Encode(response)
}

func handleMediaStream(w http.ResponseWriter, r *http.Request) {
    twilioConn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Printf("Failed to upgrade connection: %v", err)
        return
    }
    defer twilioConn.Close()

    log.Println("Client connected")

    // Connect to OpenAI WebSocket
    openAIHeader := http.Header{}
    openAIHeader.Set("Authorization", fmt.Sprintf("Bearer %s", openAIAPIKey))
    openAIHeader.Set("OpenAI-Beta", "realtime=v1")
    
    openAIConn, _, err := websocket.DefaultDialer.Dial(openAIWSURL, openAIHeader)
    if err != nil {
        log.Printf("Failed to connect to OpenAI: %v", err)
        return
    }
    defer openAIConn.Close()

    // Send initial session update
    if err := sendSessionUpdate(openAIConn); err != nil {
        log.Printf("Failed to send session update: %v", err)
        return
    }

    // Channel for coordinating goroutine shutdown
    done := make(chan struct{})
    var streamSid string
    var streamSidMutex sync.RWMutex

    // Start receiving from Twilio
    go handleTwilioMessages(twilioConn, openAIConn, &streamSid, &streamSidMutex, done)
    
    // Start sending to Twilio
    go handleOpenAIMessages(openAIConn, twilioConn, &streamSid, &streamSidMutex, done)

    // Wait for done signal
    <-done
}

func handleTwilioMessages(twilioConn, openAIConn *websocket.Conn, streamSid *string, mutex *sync.RWMutex, done chan struct{}) {
    defer close(done)
    
    for {
        _, message, err := twilioConn.ReadMessage()
        if err != nil {
            log.Printf("Error reading from Twilio: %v", err)
            return
        }

        var twilioMsg TwilioMessage
        if err := json.Unmarshal(message, &twilioMsg); err != nil {
            log.Printf("Error unmarshaling Twilio message: %v", err)
            continue
        }

        switch twilioMsg.Event {
        case "media":
            audioAppend := map[string]interface{}{
                "type":  "input_audio_buffer.append",
                "audio": twilioMsg.Media.Payload,
            }
            if err := openAIConn.WriteJSON(audioAppend); err != nil {
                log.Printf("Error sending to OpenAI: %v", err)
                return
            }
        case "start":
            mutex.Lock()
            *streamSid = twilioMsg.Start.StreamSid
            mutex.Unlock()
            log.Printf("Stream started: %s", *streamSid)
        }
    }
}

func handleOpenAIMessages(openAIConn, twilioConn *websocket.Conn, streamSid *string, mutex *sync.RWMutex, done chan struct{}) {
    for {
        select {
        case <-done:
            return
        default:
            _, message, err := openAIConn.ReadMessage()
            if err != nil {
                log.Printf("Error reading from OpenAI: %v", err)
                return
            }

            var response map[string]interface{}
            if err := json.Unmarshal(message, &response); err != nil {
                log.Printf("Error unmarshaling OpenAI message: %v", err)
                continue
            }

            // Log specific event types
            if responseType, ok := response["type"].(string); ok {
                for _, eventType := range logEventTypes {
                    if responseType == eventType {
                        log.Printf("Received event: %s %v", responseType, response)
                    }
                }
            }

            // Handle audio response
            if response["type"] == "response.audio.delta" && response["delta"] != nil {
                delta, _ := response["delta"].(string)
                audioData, err := base64.StdEncoding.DecodeString(delta)
                if err != nil {
                    log.Printf("Error decoding audio: %v", err)
                    continue
                }

                mutex.RLock()
                currentStreamSid := *streamSid
                mutex.RUnlock()

                audioDelta := map[string]interface{}{
                    "event":     "media",
                    "streamSid": currentStreamSid,
                    "media": map[string]string{
                        "payload": base64.StdEncoding.EncodeToString(audioData),
                    },
                }

                if err := twilioConn.WriteJSON(audioDelta); err != nil {
                    log.Printf("Error sending to Twilio: %v", err)
                    return
                }
            }
        }
    }
}

func sendSessionUpdate(conn *websocket.Conn) error {
    update := SessionUpdate{
        Type: "session.update",
        Session: struct {
            TurnDetection struct {
                Type string `json:"type"`
            } `json:"turn_detection"`
            InputAudioFormat  string   `json:"input_audio_format"`
            OutputAudioFormat string   `json:"output_audio_format"`
            Voice            string   `json:"voice"`
            Instructions     string   `json:"instructions"`
            Modalities      []string `json:"modalities"`
            Temperature     float64  `json:"temperature"`
        }{
            TurnDetection: struct {
                Type string `json:"type"`
            }{
                Type: "server_vad",
            },
            InputAudioFormat:  "g711_ulaw",
            OutputAudioFormat: "g711_ulaw",
            Voice:            voice,
            Instructions:     systemMessage,
            Modalities:      []string{"text", "audio"},
            Temperature:     0.8,
        },
    }

    log.Printf("Sending session update: %+v", update)
    return conn.WriteJSON(update)
}
