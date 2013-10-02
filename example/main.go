package main

import (
	redigo "github.com/garyburd/redigo/redis"
	"github.com/pierrre/imageserver"
	imageserver_cache_chain "github.com/pierrre/imageserver/cache/chain"
	imageserver_cache_memory "github.com/pierrre/imageserver/cache/memory"
	imageserver_cache_prefix "github.com/pierrre/imageserver/cache/prefix"
	imageserver_cache_redis "github.com/pierrre/imageserver/cache/redis"
	imageserver_http "github.com/pierrre/imageserver/http"
	imageserver_http_parser_graphicsmagick "github.com/pierrre/imageserver/http/parser/graphicsmagick"
	imageserver_http_parser_merge "github.com/pierrre/imageserver/http/parser/merge"
	imageserver_http_parser_source "github.com/pierrre/imageserver/http/parser/source"
	imageserver_processor_graphicsmagick "github.com/pierrre/imageserver/processor/graphicsmagick"
	imageserver_processor_limit "github.com/pierrre/imageserver/processor/limit"
	imageserver_provider_cache "github.com/pierrre/imageserver/provider/cache"
	imageserver_provider_http "github.com/pierrre/imageserver/provider/http"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	cache := imageserver_cache_chain.ChainCache{
		imageserver_cache_memory.New(10 * 1024 * 1024),
		&imageserver_cache_redis.RedisCache{
			Pool: &redigo.Pool{
				Dial: func() (redigo.Conn, error) {
					return redigo.Dial("tcp", "localhost:6379")
				},
				MaxIdle: 50,
			},
			Expire: time.Duration(7 * 24 * time.Hour),
		},
	}

	imageServer := &imageserver.Server{
		Cache: &imageserver_cache_prefix.PrefixCache{
			Prefix: "processed:",
			Cache:  cache,
		},
		Provider: &imageserver_provider_cache.CacheProvider{
			Cache: &imageserver_cache_prefix.PrefixCache{
				Prefix: "source:",
				Cache:  cache,
			},
			Provider: &imageserver_provider_http.HttpProvider{},
		},
		Processor: imageserver_processor_limit.New(16, &imageserver_processor_graphicsmagick.GraphicsMagickProcessor{
			Executable: "/usr/bin/gm",
			AllowedFormats: []string{
				"jpeg",
				"png",
				"bmp",
				"gif",
			},
			DefaultQualities: map[string]string{
				"jpeg": "85",
			},
		}),
	}

	httpImageServer := &imageserver_http.Server{
		Parser: &imageserver_http_parser_merge.MergeParser{
			&imageserver_http_parser_source.SourceParser{},
			&imageserver_http_parser_graphicsmagick.GraphicsMagickParser{},
		},
		ImageServer: imageServer,
		Expire:      time.Duration(7 * 24 * time.Hour),
		ErrFunc: func(err error, request *http.Request) {
			log.Println(err)
		},
		HeaderFunc: func(header http.Header, request *http.Request, err error) {
			header.Set("X-Hostname", hostname)
		},
	}

	http.Handle("/", httpImageServer)
	http.ListenAndServe(":8080", nil)
}
