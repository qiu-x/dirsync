## Building

	cd cmd/dirsync
	go build

## Running

### Display all options

	./dirsync -h

### Run sync

	./dirsync -hotdir hotdir -backup backup -state statefile

### Filter Logs by Date Range

	./dirsync -view -from '2025-02-09T12:30:00Z' -to '2025-02-09T12:40:00Z'

### Combine Regex with Date Range

	./dirsync -view -filter 'document.*\\.docx' -from '2025-02-09T12:00:00Z'
