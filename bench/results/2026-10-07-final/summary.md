# V8 9.0 (6f9829d) → V8 15.4 (b8ae8c5) — résultats du 2026-10-07

b8ae8c5 précède l'isolation du runtime C++ de V8 (version publiée : e63bbc8). L'isolation a été mesurée à part, à −0,05 % de geomean v8go (`../2026-10-07-cxx-isolation/`) : les chiffres ci-dessous valent pour la version publiée.

Machine : Intel Core Ultra 9 285H, 16 CPU, Docker linux/amd64 (WSL2), Go 1.27.1, gcc 12.2. La nouvelle version est compilée comme chez un consommateur (gcc + bridge précompilé, sans allocator shim, avec les optimisations du chemin JS→Go). La version gojs est cdf 9cfc650, comparée à cdf master 97ab17b. COUNT=10.

**v8go** : `v8go-il-*.txt`, mesuré en alternant ancienne et nouvelle version à chaque tour (10 tours), pour neutraliser les perturbations de la machine. Le premier passage non alterné (`v8go-benchstat.txt`) a été trop bruité côté nouvelle version (±33 à ±206 %) et n'est pas retenu.
**gojs et endurance** : passage `bench/run.sh` du même jour.

## v8go (alterné, `v8go-il-benchstat.txt`)

| Benchmark | Ancienne | Nouvelle | Écart |
|---|---|---|---|
| CallGoFromJS1000 (1000 appels JS→Go) | 729 µs | 316 µs | **−56,7 %** |
| CallGoFromJS1000Cleanup (JS→Go + Cleanup, façon gojs) | 409 µs | 358 µs | **−12,5 %** |
| CallJSFromGo | 836 ns | 740 ns | −11,4 % |
| RunScriptTrivial | 760 ns | 693 ns | −8,9 % |
| NewValueString | 344 ns | 296 ns | −14,0 % |
| ObjectSetGet | 1069 ns | 841 ns | −21,3 % |
| ValueToGoString | 1,39 µs | 1,42 µs | +2,1 % |
| NewIsolate | 896 µs | 854 µs | −4,8 % |
| NewContext | 189 µs | 122 µs | −35,8 % |
| JSONParse | 93,6 µs | 24,3 µs | −74,1 % |
| JSONStringify | 53,6 µs | 26,8 µs | −50,0 % |
| CompileLodashCold | 2,55 ms | 2,35 ms | −7,9 % |
| CompileLodashWithCodeCache | 466 µs | 410 µs | −12,0 % |
| StartupWithLodash | 5,01 ms | 3,86 ms | −23,0 % |
| JSRegex | 56,1 µs | 33,5 µs | −40,2 % |
| JSStrings | 539 µs | 436 µs | −19,1 % |
| JSArrays | 4,27 ms | 4,00 ms | −6,3 % |
| JSJSON | 1,37 ms | 0,56 ms | −59,4 % |
| JSObjects | 151 µs | 404 µs ± 59 % | +167 % (bimodal, voir plus bas) |
| Cleanup1000Values | 9,6 µs | 13,4 µs | +39,3 % |
| **geomean** | 113,0 µs | 87,8 µs | **−22,3 %** |

Toutes les lignes sont significatives (p < 0.05, n=10).

## gojs

**Chiffre à retenir : environ −6 % de geomean** (−5,90 %), mesuré en alternant baseline et nouvelle version à chaque tour (10 tours) avec les optimisations JS→Go (`../2026-10-07-callback/callback-perf.md`, section gojs ; le passage alterné précédent, sans ces optimisations, donnait −5,8 %, `../2026-10-06-noshim/`). Contre la baseline : `V8_Cleanup/*` −3 à −12 % (WriteAttributes non significatif), `V8_ColdStart/*` −4 à −9 %, `V8_CleanupOnly` +11 % (1,65 → 1,83 µs).

Le passage `bench/run.sh` de ce jour (`gojs-benchstat.txt`), **non alterné**, donne −19,5 % de geomean, mais sa baseline varie jusqu'à ±109 % selon les lignes : c'est une borne haute bruitée, à ne pas citer comme résultat.

| Benchmark (`gojs-benchstat.txt`, non alterné) | Écart |
|---|---|
| V8_Cleanup/* (une page, moteur réutilisé) | −7 % à −30 % |
| V8_ColdStart/* (création du moteur + une page) | −10 % à −46 % (WriteAttributes non significatif ; baseline bruitée) |
| V8_CleanupOnly (Cleanup seul entre deux pages) | +19,6 % (1,85 → 2,21 µs) |
| geomean | −19,5 % (borne haute bruitée ; alterné : −5,9 %) |

## Endurance (100 000 itérations)

| Test | Ancienne | Nouvelle |
|---|---|---|
| v8go TestSoakCleanup (RSS) | 59 → 72 MiB, +2,58 % sur la 2e moitié, PASS | 54 → 103 MiB, +0,62 %, PASS |
| gojs TestSoakV8Cleanup (après GC complet) | 36 → 45 MiB, +5,33 %, FAIL (marche connue de la baseline) | 48 → 58 MiB, +0,55 %, PASS |
| RSS de pointe par worker gojs | 45 MiB | 59 MiB (**+14 MiB**) |

## Lecture

- **Chemin JS→Go** (dispatchers gojs) : −57 % sur des appels en rafale, −12,5 % au rythme gojs. La régression initiale de +21 % est résorbée.
- **Cleanup** : reste +39 % en microbenchmark (`Cleanup1000Values`, +0,37 µs par page dans gojs). Le benchmark n'appelle que `ctx.Cleanup()`, qui ne vide pas la file des tâches V8 : l'écart vient de la libération des handles `Global` par V8 15 (`GlobalHandles::Destroy`, `../2026-10-07-callback/callback-perf.md`), payée une fois par page.
- **JSObjects** : bimodal dans la nouvelle version (±59 %). Hypothèse, non vérifiée : certains échantillons subissent un GC majeur de V8 15 pendant la mesure (10 000 objets créés par opération, valeurs retenues jusqu'au Cleanup). Le phénomène est déjà observé dans les campagnes précédentes (+10,7 % ± 202 %). Les pages gojs réelles n'en montrent pas trace.
- **Mémoire** : environ +14 MiB par worker gojs et environ +32 MiB de plateau par isolate de longue durée. C'est le coût fixe de V8 15, sans fuite.
