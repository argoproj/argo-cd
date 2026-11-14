#!/usr/bin/env bash

# Historically we've used mkdocs admonitions for callouts in our documentation. Those admonitions look great in mkdocs,
# but they don't render well in GitHub. GitHub supports a different style of callouts, called alerts.
# This script converts mkdocs admonitions to GitHub-flavored markdown alerts.

# GitHub alerts don't support titles, so we convert titles to bold text at the start of the alert body.

# This script is run manually. It's left here in case we need to run it again in the future.

declare -A TYPE_MAP=(
  ["note"]="NOTE"
  ["tip"]="TIP"
  ["important"]="IMPORTANT"
  ["warning"]="WARNING"
  ["danger"]="CAUTION"
)

# Process all .md files in the docs directory.
find docs -type f -name "*.md" | while read -r FILE; do
  # Skip the file if it doesn't contain any admonitions.
  if ! grep -q '^!!!' "$FILE"; then
    continue
  fi

  # Create a temporary file to store the converted content.
  TEMP_FILE=$(mktemp)

  # Read the file line by line.
  ALERT_STARTED=0
  while IFS= read -r LINE || [[ -n "$LINE" ]]; do
    # Check if the line starts with an admonition (!!!).
    if [[ "$LINE" =~ ^!!![[:space:]]*([a-zA-Z]+)([[:space:]]*\"(.*)\")? ]]; then
      TYPE=${BASH_REMATCH[1]}
      TITLE=${BASH_REMATCH[3]}

      # Map the MkDocs type to GitHub alert type.
      ALERT_TYPE=${TYPE_MAP[$TYPE]}
      if [[ -z "$ALERT_TYPE" ]]; then
        # If the type is not in the map, copy the line as is.
        echo "$LINE" >> "$TEMP_FILE"
        continue
      fi

      # Start the GitHub alert.
      echo -e "> [!$ALERT_TYPE]" >> "$TEMP_FILE"
      ALERT_STARTED=1

      # Add the title as bold text if it exists.
      if [[ -n "$TITLE" ]]; then
        echo -e "> **$TITLE**" >> "$TEMP_FILE"
        # Add a blank line after the title for better formatting.
        echo -e ">" >> "$TEMP_FILE"
      fi
    else
      # For non-admonition lines, check indentation and adjust for alert body.
      if [[ ALERT_STARTED -eq 1 ]] && [[ "$LINE" =~ ^[[:space:]]+ ]]; then
        # Strip leading whitespace and prepend "> "
        STRIPPED_LINE=${LINE##+([[:space:]])}
        echo -e "> $STRIPPED_LINE" >> "$TEMP_FILE"
      else
        echo "$LINE" >> "$TEMP_FILE"
        ALERT_STARTED=0
      fi
    fi
  done < "$FILE"

  # Replace the original file with the converted content.
  mv "$TEMP_FILE" "$FILE"
done

echo "Conversion complete."
