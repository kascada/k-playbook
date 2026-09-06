// Package scripts enthält keine Programmlogik, sondern die Tests der
// Bash-Skripte unter scripts/.
//
// Es gibt im Repo keinen Shell-Testrahmen. Die Skripte werden deshalb aus Go
// heraus über os/exec mit ihren echten Argumenten aufgerufen; damit laufen
// ihre Tests in `make test` und im Release-Preflight mit, ohne dass eine
// zweite Testmaschinerie entsteht.
//
// Die Tests kommen ohne Netzzugriff und ohne echte Installation aus: geprüft
// werden Guards, die Rangfolge der Installationswege über --dry-run und die
// Platzhalter-Auflösung gegen eine vorgegebene Asset-Liste.
package scripts
