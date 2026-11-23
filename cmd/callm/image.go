package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"callm/internal/client"
	"callm/internal/config"
	"callm/internal/ui"
)

// imageReserved lists request fields that --param must not replace: callm sets
// them itself, or depends on their default to decode the response.
var imageReserved = map[string]bool{"model": true, "prompt": true, "input_references": true, "stream": true, "response_format": true}

type imageResponse struct {
	Created int64 `json:"created"`
	Data    []struct {
		B64JSON   string `json:"b64_json"`
		URL       string `json:"url"`
		MediaType string `json:"media_type"`
	} `json:"data"`
	Usage json.RawMessage `json:"usage"`
}

type imageFile struct {
	Path      string `json:"path,omitempty"`
	URL       string `json:"url,omitempty"`
	MediaType string `json:"media_type,omitempty"`
	Bytes     int    `json:"bytes,omitempty"`
}

type imageModel struct {
	ID           string `json:"id"`
	Architecture struct {
		InputModalities []string `json:"input_modalities"`
	} `json:"architecture"`
	SupportedParameters map[string]json.RawMessage `json:"supported_parameters"`
	SupportsStreaming   bool                       `json:"supports_streaming"`
}

// runImage generates images with OpenRouter's Image API (POST /images) and
// writes them to files. Reference images travel as input_references.
func runImage(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("image", flag.ContinueOnError)
	timeout := registerTimeout(fs)
	headerTimeout := registerDuration(fs, "header-timeout", client.DefaultTimeout)
	userAgent := registerUserAgent(fs)
	stdinTimeout := registerDuration(fs, "stdin-timeout", client.DefaultTimeout)
	maxInput := registerInputLimit(fs)
	var pFlags presetFlags
	pFlags.Register(fs)
	var nFlags netFlags
	nFlags.Register(fs)
	var (
		keyFlag, keyEnvFlag, customAPI, model, output string
		aspect, resolution, size, quality, format     string
		background                                    string
		refs, params                                  stringSlice
		count, seed                                   int
		list, jsonOutput, showStats, force            bool
	)
	fs.StringVar(&keyFlag, "k", "", "API key")
	fs.StringVar(&keyFlag, "key", "", "API key")
	fs.StringVar(&keyFlag, "api-key", "", "API key")
	fs.StringVar(&keyEnvFlag, "key-env", "", "Environment variable name containing API key")
	fs.StringVar(&keyEnvFlag, "api-key-env", "", "Environment variable name containing API key")
	fs.StringVar(&customAPI, "api", "", "Custom API base URL")
	fs.StringVar(&customAPI, "base-url", "", "Custom API base URL")
	fs.StringVar(&model, "m", "", "Image model ID (required; CALLM_IMAGE_MODEL)")
	fs.StringVar(&model, "model", "", "Image model ID (required; CALLM_IMAGE_MODEL)")
	fs.StringVar(&output, "o", "", "Output file; the extension is added when missing (default callm-image-<time>)")
	fs.StringVar(&output, "output", "", "Output file; the extension is added when missing (default callm-image-<time>)")
	fs.Var(&refs, "ref", "Reference image: local file or http(s) URL (can repeat)")
	fs.Var(&refs, "image", "Alias of --ref")
	fs.StringVar(&aspect, "aspect-ratio", "", "Aspect ratio such as 1:1, 16:9 or auto")
	fs.StringVar(&resolution, "resolution", "", "Resolution tier: 512, 768, 1K, 1.5K, 2K or 4K")
	fs.StringVar(&size, "size", "", "Size shorthand: a tier or explicit pixels such as 2048x2048")
	fs.StringVar(&quality, "quality", "", "Quality: auto, low, medium or high")
	fs.StringVar(&format, "format", "", "Output format: png, jpeg, webp, svg or gif")
	fs.StringVar(&background, "background", "", "Background: auto, transparent or opaque")
	fs.IntVar(&count, "n", 0, "Number of images, 1-10")
	fs.IntVar(&count, "count", 0, "Number of images, 1-10")
	fs.IntVar(&seed, "seed", 0, "Seed for repeatable output where supported")
	fs.Var(&params, "param", "Extra request field KEY=VALUE; VALUE is JSON or a plain string (can repeat)")
	fs.BoolVar(&list, "list", false, "List image models and the parameters each accepts; no generation")
	fs.BoolVar(&jsonOutput, "json", false, "Print a JSON summary (files, usage) instead of file paths")
	fs.BoolVar(&showStats, "stats", false, "Print usage and latency to stderr")
	fs.BoolVar(&force, "force", false, "Overwrite an existing output file")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: callm image [OPTIONS] -m MODEL [\"PROMPT\"...]   (prompt from arguments or stdin)")
		fmt.Fprintln(fs.Output(), "       callm image --list [FILTER]")
		fmt.Fprintln(fs.Output(), "Uses OpenRouter's Image API (OPENROUTER_API_KEY). Every generation is billed unless the model is free.")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		die(err)
	}

	// The Image API is OpenRouter's; other presets need a compatible --base-url.
	presetName := "or"
	if pFlags != (presetFlags{}) {
		presetName = pFlags.ResolvePreset()
	}
	if presetName != "or" && customAPI == "" {
		die(errors.New("image uses OpenRouter's Image API; select it with --or, or give a compatible --base-url"))
	}
	apiKey, err := config.ResolveAPIKey(presetName, keyFlag, keyEnvFlag)
	if err != nil {
		die(err)
	}
	baseURL := nFlags.baseURL(presetName, customAPI)
	apiClient := client.NewClient(baseURL, apiKey, pFlags.clientProvider(presetName, baseURL, customAPI))
	apiClient.UserAgent = *userAgent
	nFlags.applyProxy(apiClient)
	apiClient.MaxRequestBytes = *maxInput
	apiClient.HTTPClient.Timeout = *timeout
	if !flagWasSet(fs, "header-timeout") {
		*headerTimeout = *timeout
	}
	if transport, ok := apiClient.HTTPClient.Transport.(*http.Transport); ok {
		transport.ResponseHeaderTimeout = *headerTimeout
	}

	if list {
		if fs.NArg() > 1 {
			die(errors.New("image --list accepts one optional filter"))
		}
		listImageModels(ctx, apiClient, strings.ToLower(fs.Arg(0)), jsonOutput)
		return
	}
	if apiKey == "" {
		die(missingKeyError(presetName))
	}
	if model == "" {
		model = os.Getenv("CALLM_IMAGE_MODEL")
	}
	if model == "" {
		die(errors.New("image requires -m MODEL; list the choices with: callm image --list"))
	}
	if flagWasSet(fs, "n") || flagWasSet(fs, "count") {
		if count < 1 || count > 10 {
			die(errors.New("-n must be between 1 and 10"))
		}
	}

	prompt := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if prompt == "" && !ui.IsTerminal(os.Stdin) {
		inputCtx, cancel := inputContext(ctx, *stdinTimeout)
		data, inputErr := readStdinWithLimit(inputCtx, *maxInput)
		cancel()
		if inputErr != nil {
			die(fmt.Errorf("stdin: %w", inputErr))
		}
		prompt = strings.TrimSpace(data)
	}
	if prompt == "" {
		die(errors.New("image requires a prompt"))
	}

	body := map[string]interface{}{"model": model, "prompt": prompt}
	for key, value := range map[string]string{"aspect_ratio": aspect, "resolution": resolution, "size": size, "quality": quality, "output_format": format, "background": background} {
		if value != "" {
			body[key] = value
		}
	}
	if count > 0 {
		body["n"] = count
	}
	if flagWasSet(fs, "seed") {
		body["seed"] = seed
	}
	for _, param := range params {
		key, value, ok := strings.Cut(param, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			die(fmt.Errorf("--param %q must be KEY=VALUE", param))
		}
		if imageReserved[key] {
			die(fmt.Errorf("--param cannot set %q", key))
		}
		var parsed interface{}
		if json.Unmarshal([]byte(value), &parsed) != nil {
			parsed = value
		}
		body[key] = parsed
	}
	if len(refs) > 0 {
		references := make([]map[string]interface{}, 0, len(refs))
		for _, ref := range refs {
			uri, encodeErr := encodeImageToDataURI(ref)
			if encodeErr != nil {
				die(fmt.Errorf("failed to encode image '%s': %w", ref, encodeErr))
			}
			references = append(references, map[string]interface{}{"type": "image_url", "image_url": map[string]string{"url": uri}})
		}
		body["input_references"] = references
	}
	payload, err := json.Marshal(body)
	if err != nil {
		die(err)
	}

	// Check the destination before the request: a generation that cannot be saved is still billed.
	if output == "" {
		output = "callm-image-" + time.Now().Format("20060102-150405")
	}
	directory := filepath.Dir(output)
	if info, statErr := os.Stat(directory); statErr != nil || !info.IsDir() {
		die(fmt.Errorf("output directory %q does not exist", directory))
	}
	if info, statErr := os.Stat(output); statErr == nil {
		if info.IsDir() {
			die(fmt.Errorf("output %q is a directory; give a file name", output))
		}
		if !force {
			die(fmt.Errorf("output %q exists; use --force to overwrite", output))
		}
	}

	started := time.Now()
	respBytes, err := apiClient.RawRequest(ctx, "/images", payload)
	if err != nil {
		die(err)
	}
	elapsed := time.Since(started)
	var response imageResponse
	if err := json.Unmarshal(respBytes, &response); err != nil {
		die(fmt.Errorf("image response is not valid JSON: %w", err))
	}
	if len(response.Data) == 0 {
		die(errors.New("image response contains no images"))
	}

	var files []imageFile
	for index, item := range response.Data {
		if item.B64JSON == "" {
			if item.URL == "" {
				die(fmt.Errorf("image %d has no data", index+1))
			}
			files = append(files, imageFile{URL: item.URL, MediaType: item.MediaType})
			continue
		}
		data, decodeErr := base64.StdEncoding.DecodeString(item.B64JSON)
		if decodeErr != nil {
			die(fmt.Errorf("image %d is not valid base64: %w", index+1, decodeErr))
		}
		media := item.MediaType
		if media == "" {
			media = http.DetectContentType(data)
		}
		path, writeErr := writeImage(imagePath(output, index, len(response.Data), media), data, force)
		if writeErr != nil {
			die(writeErr)
		}
		files = append(files, imageFile{Path: path, MediaType: media, Bytes: len(data)})
	}

	if jsonOutput {
		summary := map[string]interface{}{"model": model, "files": files}
		if response.Created != 0 {
			summary["created"] = response.Created
		}
		if len(response.Usage) > 0 {
			summary["usage"] = response.Usage
		}
		encoded, _ := json.MarshalIndent(summary, "", "  ")
		if _, err := fmt.Println(string(encoded)); err != nil {
			die(err)
		}
	} else {
		for _, file := range files {
			line := file.Path
			if line == "" {
				line = file.URL
			}
			if _, err := fmt.Println(line); err != nil {
				die(err)
			}
		}
	}
	if showStats {
		line := fmt.Sprintf("[stats] model=%s images=%d latency=%.1fs", model, len(files), elapsed.Seconds())
		var usage struct {
			TotalTokens *int     `json:"total_tokens"`
			Cost        *float64 `json:"cost"`
		}
		if len(response.Usage) > 0 && json.Unmarshal(response.Usage, &usage) == nil {
			if usage.TotalTokens != nil {
				line += fmt.Sprintf(" tokens=%d", *usage.TotalTokens)
			}
			if usage.Cost != nil {
				line += " cost=$" + strconv.FormatFloat(*usage.Cost, 'f', -1, 64)
			}
		}
		fmt.Fprintln(os.Stderr, line)
	}
}

// imageExtension maps a media type to a file extension; unknown types keep the bytes as .bin.
func imageExtension(media string) string {
	switch strings.ToLower(strings.TrimSpace(strings.Split(media, ";")[0])) {
	case "image/png":
		return ".png"
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "image/svg+xml":
		return ".svg"
	}
	return ".bin"
}

// imagePath numbers the files of a multi-image result and adds a missing extension.
func imagePath(output string, index, total int, media string) string {
	extension := filepath.Ext(output)
	base := strings.TrimSuffix(output, extension)
	wanted := imageExtension(media)
	if extension == "" {
		extension = wanted
	} else if actual := strings.ToLower(extension); actual != wanted && !(actual == ".jpeg" && wanted == ".jpg") && wanted != ".bin" {
		fmt.Fprintf(os.Stderr, "callm: the image is %s but the file name ends in %s\n", media, extension)
	}
	if total > 1 {
		base += "-" + strconv.Itoa(index+1)
	}
	return base + extension
}

// writeImage never replaces a file without --force. The image is already paid
// for, so a name taken since the first check gets a numbered sibling instead.
func writeImage(path string, data []byte, force bool) (string, error) {
	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	if force {
		flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}
	extension := filepath.Ext(path)
	base := strings.TrimSuffix(path, extension)
	candidate := path
	for attempt := 1; ; attempt++ {
		file, err := os.OpenFile(candidate, flags, 0o644)
		if err == nil {
			_, writeErr := file.Write(data)
			if closeErr := file.Close(); writeErr == nil {
				writeErr = closeErr
			}
			if writeErr != nil {
				return "", fmt.Errorf("failed to write %s: %w", candidate, writeErr)
			}
			if candidate != path {
				fmt.Fprintf(os.Stderr, "callm: %s exists; saved as %s\n", path, candidate)
			}
			return candidate, nil
		}
		if !errors.Is(err, os.ErrExist) || attempt > 99 {
			return "", fmt.Errorf("failed to write %s: %w", candidate, err)
		}
		candidate = fmt.Sprintf("%s.%d%s", base, attempt, extension)
	}
}

func listImageModels(ctx context.Context, apiClient *client.Client, filter string, asJSON bool) {
	respBytes, err := apiClient.GetRaw(ctx, "/images/models")
	if err != nil {
		die(err)
	}
	var catalog struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(respBytes, &catalog); err != nil {
		die(fmt.Errorf("image model list is not valid JSON: %w", err))
	}
	var kept []json.RawMessage
	var models []imageModel
	for _, raw := range catalog.Data {
		var model imageModel
		if json.Unmarshal(raw, &model) != nil || model.ID == "" {
			continue
		}
		if filter != "" && !strings.Contains(strings.ToLower(model.ID), filter) {
			continue
		}
		kept = append(kept, raw)
		models = append(models, model)
	}
	if asJSON {
		encoded, _ := json.MarshalIndent(map[string]interface{}{"data": kept}, "", "  ")
		if _, err := fmt.Println(string(encoded)); err != nil {
			die(err)
		}
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "MODEL\tREFERENCES\tPARAMETERS")
	for _, model := range models {
		references := "no"
		keys := make([]string, 0, len(model.SupportedParameters))
		for key := range model.SupportedParameters {
			if key == "input_references" {
				references = "yes"
				continue
			}
			keys = append(keys, key)
		}
		sort.Strings(keys)
		fmt.Fprintf(w, "%s\t%s\t%s\n", model.ID, references, strings.Join(keys, ","))
	}
	if err := w.Flush(); err != nil {
		die(err)
	}
}
