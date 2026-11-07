package mcpserver

// errorEnvelope ist der Fehlerumschlag aller Werkzeugantworten: ein Code, an
// dem der Aufrufer entscheidet, ob er die Argumente korrigiert oder aufgibt,
// und die Meldung im Klartext.
type errorEnvelope struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
