# text_exporter

Sometimes you just need a text exporter.  You can certainly configure node_exporter for text export, but maybe you also need different scrape timing for different metrics. 

    go build -ldflags="-X main.commit=$(git rev-parse --short HEAD) -X main.branch=$(git branch | sed 's/.*\* //')"

    modify / install the appropriate unit file, and launch