// Package program kümmert sich um die Programmdatei selbst: ob das laufende
// Programm älter ist als das neueste Release, ob `k-playbook` im PATH auf das
// Installationsziel zeigt, welche Version eine installierte Datei trägt, und
// wie das Release-Programm des neuesten Tags installiert wird.
//
// Es gibt genau zwei Versionsquellen: die eigene Version im Binary und den
// neuesten Release-Tag auf GitHub (Paket release). Der Clone k-playbook/ des
// Projekts spielt dabei keine Rolle mehr — weder seine VERSION noch seine
// SHA256SUMS noch sein bin/install werden gelesen oder aufgerufen. Geladen,
// geprüft und ersetzt wird über release.Install; dieses Paket legt davor die
// PATH-Prüfung und den Blick auf die Datei am Ziel.
//
// Dazu Kephalaion: seine Installation über das install.sh des neuesten
// Kephalaion-Releases und die Aufrufe von kephalaion upgrade.
package program
