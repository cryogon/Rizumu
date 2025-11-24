#!/bin/bash

# Usage: ./test_stream.sh <song_id>
# Example: ./test_stream.sh 1

SONG_ID=${1:-1}
URL="http://localhost:8080/stream/$SONG_ID"

echo "🎧 Testing Stream for Song ID: $SONG_ID"
echo "👉 Target: $URL"

while true; do
  # 1. Check HTTP Status Code
  STATUS=$(curl -o /dev/null -s -w "%{http_code}" "$URL")

  if [ "$STATUS" == "200" ]; then
    echo -e "\n✅ 200 OK! Song is ready."

    # 2. Player Selection Logic
    if command -v mpv &>/dev/null; then
      echo "🚀 Playing with mpv..."
      mpv "$URL"

    elif command -v cvlc &>/dev/null; then
      echo "🚀 Playing with cvlc..."
      cvlc "$URL" --play-and-exit

    elif command -v vlc &>/dev/null; then
      echo "🚀 Playing with vlc (headless)..."
      # -I dummy prevents the GUI from opening
      # --play-and-exit closes vlc when the song finishes
      vlc -I dummy "$URL" --play-and-exit

    elif command -v ffplay &>/dev/null; then
      echo "🚀 Playing with ffplay..."
      ffplay -autoexit -nodisp "$URL"

    else
      echo "⚠️ No player found (mpv/vlc/ffplay). Open this URL in your browser:"
      echo "$URL"
    fi
    break

  elif [ "$STATUS" == "202" ]; then
    echo -ne "⏳ 202 Accepted: Backend is buffering/downloading...   \r"
    sleep 2

  elif [ "$STATUS" == "404" ]; then
    echo -e "\n❌ 404 Not Found. Does this Song ID exist in your DB?"
    break

  else
    echo -e "\n⚠️ Unexpected Status: $STATUS"
    curl -s "$URL"
    echo ""
    break
  fi
done
