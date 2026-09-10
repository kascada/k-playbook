# Regel: Entscheidungen über den Zweck gehören dem Nutzer

## Zweck

Eine Entscheidung, die sich aus Code, Doku und Messung ableiten lässt, trifft die KI. Eine
Entscheidung, die davon abhängt, wofür etwas gedacht ist, trifft der Nutzer. Er kennt den
Plan; die KI sieht nur den Stand.

Diese Regel zieht die Grenze und sagt, was zu tun ist, wenn sie überschritten wird.

## Woran eine solche Entscheidung zu erkennen ist

Die Frage lautet: **Würde ein anderer Zweck zu einer anderen Antwort führen?** Wenn ja,
gehört sie dem Nutzer. Typische Fälle:

- **Verfahrenswahl.** Ein Index, ein Format, eine Bibliothek, ein Ablageort, wo mehrere
  vertretbar sind und die Wahl vom erwarteten Umfang, von der Betriebsumgebung oder vom
  Publikum abhängt.
- **Umfang.** Wächst etwas auf Hunderte oder auf Hunderttausende? Die KI kann den Ist-Stand
  zählen, aber nicht wissen, was noch kommen soll.
- **Wer der Abnehmer ist.** Ein Mensch in der Oberfläche und eine KI-Sitzung stellen
  verschiedene Anforderungen an dieselbe Schnittstelle.
- **Was ein Artefakt beweisen soll.** Eine Messung, ein Test, ein Bericht ist nur dann
  etwas wert, wenn feststeht, welche Frage er beantwortet.
- **Abgrenzung.** Gehört etwas in diesen Schritt oder in den nächsten, und was fällt
  ersatzlos weg.

Nicht gemeint sind Implementierungsdetails, die der Vertrag nicht berührt: Benennung
innerhalb einer Datei, Aufteilung in Funktionen, Wahl der Datenstruktur, Testaufbau. Die
trifft die KI selbst und fragt nicht.

## Was zu tun ist

Vor der Arbeit, nicht danach: die Entscheidung benennen, die Möglichkeiten nebeneinander
stellen, sagen was jede kostet und was sie ausschließt, und eine Empfehlung abgeben. Dann
warten.

Ein Task, der eine solche Entscheidung schon getroffen hat, entbindet nicht — er verschiebt
sie nur. Steht sie im Task und ist zweifelhaft geworden, ist das vor der Ausführung zu
sagen, nicht im Abschlussvermerk.

## Was nicht genügt

- **Eine Warnung statt einer Frage.** „Ausdrücklich eine Untergrenze" neben eine Zahl zu
  schreiben, die die Frage nicht beantwortet, ersetzt nicht die Frage, ob die Messung
  überhaupt gewollt ist. Ein Vorbehalt macht ein untaugliches Ergebnis nicht tauglich.
- **Den Vorbehalt in ein Artefakt schreiben.** Ein Satz im Abschlussvermerk erreicht den
  Nutzer erst, wenn die Arbeit getan ist. Dann ist die Entscheidung gefallen, nur eben
  nicht von ihm.
- **Sich auf einen Task berufen.** Auch ein gegengelesener Task ist eine Vorlage und kein
  Auftraggeber. Wenn sein Review-Protokoll den Zweifel bereits vermerkt, ist das ein Grund
  mehr zu fragen, nicht weniger.
- **Nachträglich vorlegen.** Nach der Ausführung ist die Frage keine Frage mehr, sondern
  eine Rechtfertigung.

## Herkunft

Aufgenommen am 2026-09-10 nach Task 055. Der Task verlangte, die Größe des Wissensbestands
zu messen, um die Entscheidung über einen Vektorindex vorzubereiten. Sein Review-Protokoll
hielt bereits fest, dass dieses Repo dafür zu klein ist. Gemessen wurden 46 Chunks, bei
Schwellen von einigen tausend und einigen zehntausend — eine Zahl, die zwischen den
Möglichkeiten nicht unterscheidet.

Die ausführende Sitzung hat den Vorbehalt weitergetragen und ausgeführt, statt zu fragen,
ob die Messung ihren Zweck hier überhaupt erfüllen kann. Der Nutzer hat die Zahl
anschließend als nicht aussagekräftig zurückgewiesen, mit der Begründung, dass er den Plan
kennt, wofür das Ganze gedacht ist. Genau das ist der Punkt: die Frage war nie technisch.
