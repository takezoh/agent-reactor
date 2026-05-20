// Code generated from JSON Schema using quicktype. DO NOT EDIT.
// To parse and unparse this JSON data, add this code to your project and do:
//
//    initializeParams, err := UnmarshalInitializeParams(bytes)
//    bytes, err = initializeParams.Marshal()
//
//    initializeResponse, err := UnmarshalInitializeResponse(bytes)
//    bytes, err = initializeResponse.Marshal()

package codexschemav1

import "encoding/json"

func UnmarshalInitializeParams(data []byte) (InitializeParams, error) {
	var r InitializeParams
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *InitializeParams) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalInitializeResponse(data []byte) (InitializeResponse, error) {
	var r InitializeResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *InitializeResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type InitializeParams struct {
	Capabilities *InitializeCapabilities `json:"capabilities"`
	ClientInfo   ClientInfo              `json:"clientInfo"`
}

// Client-declared capabilities negotiated during initialize.
type InitializeCapabilities struct {
	// Opt into receiving experimental API methods and fields.                                    
	ExperimentalAPI                                                                      *bool    `json:"experimentalApi,omitempty"`
	// Exact notification method names that should be suppressed for this connection (for         
	// example `thread/started`).                                                                 
	OptOutNotificationMethods                                                            []string `json:"optOutNotificationMethods"`
}

type ClientInfo struct {
	Name    string  `json:"name"`
	Title   *string `json:"title"`
	Version string  `json:"version"`
}

type InitializeResponse struct {
	// Absolute path to the server's $CODEX_HOME directory.                                           
	CodexHome                                                                                  string `json:"codexHome"`
	// Platform family for the running app-server target, for example `"unix"` or `"windows"`.        
	PlatformFamily                                                                             string `json:"platformFamily"`
	// Operating system for the running app-server target, for example `"macos"`, `"linux"`, or       
	// `"windows"`.                                                                                   
	PlatformOS                                                                                 string `json:"platformOs"`
	UserAgent                                                                                  string `json:"userAgent"`
}
