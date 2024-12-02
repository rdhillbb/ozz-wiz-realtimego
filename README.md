# Ozz-Wiz-RealtimeGo

A real-time audio integration application that connects Twilio with OpenAI's Realtime Audio API. The application utilizes Go routines for efficient asynchronous processing of audio streams between both services.

## Features

- Seamless integration between Twilio and OpenAI Realtime Audio API
- High-performance asynchronous processing using Go routines
- Configurable messaging system through XML
- Real-time audio streaming and processing
- Flexible protocol support (HTTP/HTTPS)

## Configuration

### Environment Variables

| Variable | Description | Required |
|----------|-------------|----------|
| `OPENAI_API_KEY` | Your OpenAI API authentication key | Yes |
| `USEHTTP` | Communication protocol (`HTTP` or `HTTPS`) | Yes |
| `XMLMESSAGE` | Path to XML message configuration file | Yes |

#### Note:<br>
###### Environment variables are to be place in .env where geppettoaudio is executed.<br>

### Message Configuration

The application uses an XML file to define welcome and system messages. Place your configuration file at the path specified in the `XMLMESSAGE` environment variable.

Example configuration (`ozzmsg.xml`):

```xml
<osswiz>
    <intromessages>
        <welcomemessage>
            Welcome to our audio processing service. How may we assist you today?
        </welcomemessage>
    </intromessages>
    <systemmsg>
        You are now connected to our AI assistant. Please speak clearly.
    </systemmsg>
</osswiz>
```

## Getting Started

1. Set up your environment variables:
   ```bash
   export OPENAI_API_KEY="your_api_key"
   export USEHTTP="HTTPS"
   export XMLMESSAGE="./ozzmsg.xml"
   ```

2. Create your XML message configuration file according to the format above

3. Run the application:
   ```bash
   go run geppettoaudio.go
   ```
