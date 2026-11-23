---
title: Befunde als Todo
description: Modul für /k-task-run, vom Hauptkontext gelesen, wenn der Lauf Review-Befunde gesammelt hat — die einmalige Abfrage am Ende (Antwortformen alle, keine, alle außer, nur), das Anlegen der Todos mit k_playbook_todo_add und der Umgang mit outcome_unknown.
---

# Befunde als Todo

Dieses Modul liest der Hauptkontext von `/k-task-run` in Schritt 7, wenn er Review-Befunde
gesammelt hat. Er hat sie je Task als Zeilen `<datei:zeile oder Prüfbefehl> — <ein Satz>`,
kritische mit dem Merkmal „kritisch“, dazu je Task `TASK_FILE`, `TASK_REF_DONE` und die
Nummer des Tasks.

Hinweise (`Hinweise:`) gehören nicht dazu; sie stehen nur in der Zusammenfassung.

Die Abfrage kommt genau einmal je Lauf, auch wenn der Lauf vorher anhielt oder abbrach (5e, 5g,
Stopp-Abfrage (c), `foreign_change`); sie gilt den Befunden der bis dahin abgeschlossenen
Tasks. Erst die Fertig-Meldung
(`Rückfrage: Befunde als Todo`, der Body nennt die Zahl der Befunde), dann in **einer**
Nachricht alle Befunde, nummeriert über den ganzen Lauf, nach Tasks gegliedert:

```
Review-Befunde aus diesem Lauf:
  <TASK_FILE>
    1. kritisch: <datei:zeile oder Prüfbefehl> — <ein Satz>
    2. <datei:zeile oder Prüfbefehl> — <ein Satz>
  <TASK_FILE>
    3. <datei:zeile oder Prüfbefehl> — <ein Satz>

Als Todo übernehmen? (alle / keine / alle außer <nummern> / nur <nummern>)
```

Voreingestellt ist `alle`: Eine Antwort wie „ja“ oder „ok“ gilt als `alle`. `<nummern>` sind
Nummern aus der Liste, getrennt durch Komma oder Leerzeichen, auch Bereiche wie `2-4`. Passt
eine Antwort auf keine der vier Formen oder nennt sie eine Nummer, die nicht in der Liste
steht, wird nichts angelegt und einmal nachgefragt, wie sie gemeint ist.

Je übernommenem Befund ein `k_playbook_todo_add` (`projectDir` = `context.project.dir`):

- `text` ist der Befund in einem Satz samt Ort (`datei:zeile` oder Prüfbefehl), so
  formuliert, dass er ohne Task und Lauf verständlich ist; einen kritischen kennzeichnet der
  Satz als „Review-Befund kritisch“.
- `origin` ist `Task <NNN> — <TASK_REF_DONE>`, mit der Nummer des Tasks und seinem Verweis
  unter `done/`.
- Kein Verweis auf `keph:/queue/`.
- `outcome_unknown`: nicht wiederholen, sondern mit `k_playbook_todo_list` und `search` (ein
  markantes Stück aus `text`) nachsehen; steht das Todo da, gilt seine Kennung, sonst wird es
  einmal neu angelegt.

Die angelegten Kennungen nennt die Zusammenfassung in der Zeile „Todos“.
