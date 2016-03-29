# Insert license header at the top of each file 
# NOTE: this script has to be executed from the project root
find src/ -name "*.go" | while read fileName
do 
  if ! grep -q Copyright "$fileName"
  then
    echo Inserting license in $fileName
    cat Tools/src/LICENSE > "$fileName.new"
    
    # add empty line after the license, in case the license doesn't have one
    # (gofmt will clear it if not needed)
    echo "" >> "$fileName.new"
    
    cat "$fileName" >> "$fileName.new"
    mv "$fileName.new" "$fileName"
  fi
done

