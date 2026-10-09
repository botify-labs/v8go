# V8 9.0 (6f9829d) → V8 15.4.80.20 (deps/v8_hash b3d6849), sans allocator shim — résultats du 2026-10-06

Machine : Intel Core Ultra 9 285H, 16 CPU visibles (Docker Desktop / WSL2, 31 GiB) — COUNT=10 — linux/amd64 (Docker, Go 1.27.1, gcc 12.2), nouvelle version compilée comme chez un consommateur (gcc + bridge précompilé). v8go : branche de pré-publication à la date de la campagne (libs V8 sans l'*allocator shim* de PartitionAlloc, retiré par `tools/sync_tommie.sh`), baseline 6f9829d. `BenchmarkCleanup1000Values` tourne à part avec `-benchtime=2000x`, mêmes réglages pour les deux versions. Détails : `env-213246.txt`.

La campagne précédente, **avec** le shim, est `../2026-10-06/`. Cette campagne la remplace.

## Avec shim / sans shim

Les deux campagnes ont tourné sur la même machine, à quelques heures d'écart. La baseline elle-même a varié entre les deux passes : geomean -3.7 % (`v8go-baseline-drift.txt`). La comparaison directe des deux « new » (`v8go-shim-vs-noshim.txt`) inclut cette dérive. La comparaison fiable est donc l'écart new/baseline **mesuré dans chaque passe** :

| Benchmark | new/baseline avec shim (`../2026-10-06/`) | new/baseline sans shim (cette passe) | new sans shim / new avec shim (direct, dérive incluse) |
|---|---|---|---|
| NewIsolate | +7.4 % | **-5.7 %** | -14.2 % |
| RunScriptTrivial | +15.8 % | **-10.5 %** | -21.7 % |
| CallJSFromGo | +40.5 % | **+2.8 %** | -28.1 % |
| CallGoFromJS1000 | +38.9 % | **+20.9 %** | -16.0 % |
| NewValueString | +22.8 % | **-6.1 %** | -24.9 % |
| ValueToGoString | +32.5 % | **+2.6 %** | -23.8 % |
| ObjectSetGet | +20.8 % | **-18.0 %** | -31.5 % |
| CompileLodashCold | +19.5 % | **-8.1 %** | -26.7 % |
| CompileLodashWithCodeCache | +21.4 % | **-11.9 %** | -30.8 % |
| JSArrays | +27.1 % | **-6.6 %** | -26.7 % |
| Cleanup1000Values | +278.5 % | **+123.8 %** | -46.5 % |
| geomean v8go | -2.6 % | **-18.8 %** | -19.7 % |

Toutes les valeurs de la colonne « sans shim » sont significatives (p < 0.05, n=10).

## v8go
```
goos: linux
goarch: amd64
pkg: github.com/botify-labs/v8go/bench
cpu: Intel(R) Core(TM) Ultra 9 285H
                              │   baseline    │                  new                   │
                              │    sec/op     │    sec/op      vs base                 │
NewIsolate-16                   1001.0µ ±  1%   944.2µ ±   2%    -5.67% (p=0.000 n=10)
NewContext-16                    208.2µ ±  1%   127.7µ ±   2%   -38.66% (p=0.000 n=10)
RunScriptTrivial-16              852.8n ±  1%   763.1n ±   2%   -10.52% (p=0.000 n=10)
CallJSFromGo-16                  910.7n ±  2%   936.3n ±   2%    +2.81% (p=0.000 n=10)
CallGoFromJS1000-16              744.2µ ±  2%   899.4µ ±   3%   +20.86% (p=0.000 n=10)
NewValueString-16                376.2n ±  1%   353.5n ±   2%    -6.05% (p=0.000 n=10)
ValueToGoString-16               1.538µ ±  1%   1.578µ ±   1%    +2.60% (p=0.000 n=10)
ObjectSetGet-16                 1196.5n ±  1%   980.8n ±   2%   -18.03% (p=0.000 n=10)
JSONParse-16                    100.91µ ±  2%   26.47µ ±   1%   -73.77% (p=0.000 n=10)
JSONStringify-16                 58.48µ ±  1%   28.80µ ±   1%   -50.75% (p=0.000 n=10)
CompileLodashCold-16             2.770m ±  2%   2.545m ±   1%    -8.12% (p=0.000 n=10)
CompileLodashWithCodeCache-16    501.8µ ±  1%   442.1µ ±   1%   -11.91% (p=0.000 n=10)
StartupWithLodash-16             5.440m ±  1%   4.228m ±   1%   -22.29% (p=0.000 n=10)
JSRegex-16                       48.87µ ± 44%   35.79µ ±  15%   -26.77% (p=0.000 n=10)
JSObjects-16                     166.1µ ±  5%   183.9µ ± 202%   +10.73% (p=0.000 n=10)
JSArrays-16                      4.712m ±  0%   4.400m ±   1%    -6.62% (p=0.000 n=10)
JSStrings-16                     572.7µ ±  1%   481.7µ ±   1%   -15.88% (p=0.000 n=10)
JSJSON-16                       1527.5µ ±  2%   618.7µ ±   3%   -59.50% (p=0.000 n=10)
HeapPerIsolate-16                1.246m ±  1%   1.083m ±   2%   -13.10% (p=0.000 n=10)
HeapPerContext-16               11.299m ±  1%   6.861m ±   2%   -39.28% (p=0.000 n=10)
Cleanup1000Values-16             10.63µ ±  1%   23.78µ ±   1%  +123.75% (p=0.000 n=10)
geomean                          114.9µ         93.33µ          -18.81%

                              │    baseline    │                  new                   │
                              │      B/op      │     B/op      vs base                  │
NewIsolate-16                     144.0 ± 0%       176.0 ± 0%  +22.22% (p=0.000 n=10)
NewContext-16                     80.00 ± 0%       80.00 ± 0%        ~ (p=1.000 n=10) ¹
RunScriptTrivial-16               16.00 ± 0%       16.00 ± 0%        ~ (p=1.000 n=10) ¹
CallJSFromGo-16                   68.00 ± 0%       68.00 ± 0%        ~ (p=1.000 n=10) ¹
CallGoFromJS1000-16             135.7Ki ± 0%     135.7Ki ± 0%        ~ (p=0.825 n=10)
NewValueString-16                 32.00 ± 0%       32.00 ± 0%        ~ (p=1.000 n=10) ¹
ValueToGoString-16                272.0 ± 0%       272.0 ± 0%        ~ (p=1.000 n=10) ¹
ObjectSetGet-16                   36.00 ± 0%       36.00 ± 0%        ~ (p=1.000 n=10) ¹
JSONParse-16                      16.00 ± 0%       16.00 ± 0%        ~ (p=1.000 n=10) ¹
JSONStringify-16                16.00Ki ± 0%     16.00Ki ± 0%        ~ (p=1.000 n=10) ¹
CompileLodashCold-16            2.491Ki ± 7%     2.326Ki ± 2%   -6.62% (p=0.000 n=10)
CompileLodashWithCodeCache-16     16.00 ± 0%       16.00 ± 0%        ~ (p=1.000 n=10) ¹
StartupWithLodash-16              272.0 ± 0%       304.0 ± 0%  +11.76% (p=0.000 n=10)
JSRegex-16                        16.00 ± 0%       16.00 ± 0%        ~ (p=1.000 n=10) ¹
JSObjects-16                      16.00 ± 0%       16.00 ± 0%        ~ (p=1.000 n=10) ¹
JSArrays-16                       16.00 ± 0%       16.00 ± 0%        ~ (p=1.000 n=10) ¹
JSStrings-16                      16.00 ± 0%       16.00 ± 0%        ~ (p=1.000 n=10) ¹
JSJSON-16                         16.00 ± 0%       16.00 ± 0%        ~ (p=1.000 n=10) ¹
HeapPerIsolate-16                 240.0 ± 0%       272.0 ± 0%  +13.33% (p=0.000 n=10)
HeapPerContext-16               4.828Ki ± 0%     4.859Ki ± 0%   +0.65% (p=0.000 n=10)
Cleanup1000Values-16              0.000 ± 0%       0.000 ± 0%        ~ (p=1.000 n=10) ¹
geomean                                      ²                  +1.80%                ²
¹ all samples are equal
² summaries must be >0 to compute geomean

                              │   baseline    │                  new                  │
                              │   allocs/op   │  allocs/op   vs base                  │
NewIsolate-16                    4.000 ± 0%      5.000 ± 0%  +25.00% (p=0.000 n=10)
NewContext-16                    5.000 ± 0%      5.000 ± 0%        ~ (p=1.000 n=10) ¹
RunScriptTrivial-16              1.000 ± 0%      1.000 ± 0%        ~ (p=1.000 n=10) ¹
CallJSFromGo-16                  5.000 ± 0%      5.000 ± 0%        ~ (p=1.000 n=10) ¹
CallGoFromJS1000-16             7.746k ± 0%     7.746k ± 0%        ~ (p=1.000 n=10) ¹
NewValueString-16                2.000 ± 0%      2.000 ± 0%        ~ (p=1.000 n=10) ¹
ValueToGoString-16               2.000 ± 0%      2.000 ± 0%        ~ (p=1.000 n=10) ¹
ObjectSetGet-16                  2.000 ± 0%      2.000 ± 0%        ~ (p=1.000 n=10) ¹
JSONParse-16                     1.000 ± 0%      1.000 ± 0%        ~ (p=1.000 n=10) ¹
JSONStringify-16                 1.000 ± 0%      1.000 ± 0%        ~ (p=1.000 n=10) ¹
CompileLodashCold-16             1.000 ± 0%      1.000 ± 0%        ~ (p=1.000 n=10) ¹
CompileLodashWithCodeCache-16    1.000 ± 0%      1.000 ± 0%        ~ (p=1.000 n=10) ¹
StartupWithLodash-16             12.00 ± 0%      13.00 ± 0%   +8.33% (p=0.000 n=10)
JSRegex-16                       1.000 ± 0%      1.000 ± 0%        ~ (p=1.000 n=10) ¹
JSObjects-16                     1.000 ± 0%      1.000 ± 0%        ~ (p=1.000 n=10) ¹
JSArrays-16                      1.000 ± 0%      1.000 ± 0%        ~ (p=1.000 n=10) ¹
JSStrings-16                     1.000 ± 0%      1.000 ± 0%        ~ (p=1.000 n=10) ¹
JSJSON-16                        1.000 ± 0%      1.000 ± 0%        ~ (p=1.000 n=10) ¹
HeapPerIsolate-16                10.00 ± 0%      11.00 ± 0%  +10.00% (p=0.000 n=10)
HeapPerContext-16                304.0 ± 0%      305.0 ± 0%   +0.33% (p=0.000 n=10)
Cleanup1000Values-16             0.000 ± 0%      0.000 ± 0%        ~ (p=1.000 n=10) ¹
geomean                                     ²                 +1.93%                ²
¹ all samples are equal
² summaries must be >0 to compute geomean

                              │   baseline   │                  new                  │
                              │     B/s      │     B/s       vs base                 │
JSONParse-16                    141.8Mi ± 2%   540.5Mi ± 1%  +281.26% (p=0.000 n=10)
JSONStringify-16                244.6Mi ± 1%   496.7Mi ± 1%  +103.03% (p=0.000 n=10)
CompileLodashCold-16            187.3Mi ± 2%   203.9Mi ± 1%    +8.84% (p=0.000 n=10)
CompileLodashWithCodeCache-16   1.010Gi ± 1%   1.146Gi ± 1%   +13.52% (p=0.000 n=10)
geomean                         286.3Mi        503.5Mi        +75.86%

                  │  baseline   │                 new                 │
                  │  heapB/iso  │  heapB/iso   vs base                │
HeapPerIsolate-16   744.6k ± 0%   191.4k ± 0%  -74.29% (p=0.000 n=10)

                  │  baseline   │                 new                 │
                  │  heapB/ctx  │  heapB/ctx   vs base                │
HeapPerContext-16   69.83k ± 0%   88.31k ± 0%  +26.46% (p=0.000 n=10)
```

## Endurance (100 000 itérations)
| Test | Version | RSS début → fin | Croissance 2e moitié | Verdict |
|---|---|---|---|---|
| v8go TestSoakCleanup | baseline | 58 → 72 MiB | 2.09 % | PASS |
| v8go TestSoakCleanup | new | 55 → 104 MiB | 0.90 % | PASS |

La nouvelle version passe le soak. `bench/run.sh` distingue un échec de soak (`--- FAIL`) d'une erreur de compilation ou d'installation.

## Mémoire
| Mesure | baseline | new | Écart |
|---|---|---|---|
| RSS plateau du soak v8go (1 isolate + 1 contexte réutilisés) | ~72 MiB | ~104 MiB | +32 MiB |
| heapB/iso (tas V8 utilisé, isolate neuf + 1 contexte) | 744.6k | 191.4k | -74 % |
| heapB/ctx (tas V8 ajouté par contexte supplémentaire) | 69.83k | 88.31k | +26 % |

Retirer le shim ne réduit pas le RSS (campagne avec shim, `../2026-10-06/` : 104 MiB avec ou sans shim ; même chose ici). Le surcoût vient de V8 15 lui-même (tas, cage sandbox/pointer compression, code). heapB/* est lu sans GC préalable : valeurs indicatives. La baisse de heapB/iso tient probablement à la comptabilité de V8 15 (espace read-only hors de UsedHeapSize), ce qui n'est pas vérifié.

## Régressions restantes > 5 % (p < 0.05)

- **CallGoFromJS1000 : +20.9 %** dans cette passe (baseline 744.2 µs ± 2 %, new 899.4 µs ± 3 %). L'A/B sans shim de la campagne précédente (`../2026-10-06/`), une autre passe, donnait +18.3 %. L'écart entre les deux passes est donc d'environ 3 points. Le shim n'explique pas cette régression : elle persiste sans lui. Cause probable, non mesurée : le chemin callback JS→Go de V8 15 et du bridge (par argument, un `std::vector` sur le tas au lieu d'un VLA et un `unordered_map` de suivi au lieu d'un `vector`, plus un retour Go à deux valeurs).
- **Cleanup1000Values : +123.8 %**. C'est le coût voulu du correctif qui fait pomper à `Isolate.Cleanup` les tâches de la plateforme V8, sans lequel la mémoire fuit. S'ajoute le parcours de l'`unordered_map`. Ce coût est payé une fois par appel de `Cleanup` entre deux exécutions, hors des benchmarks par appel.
- **JSObjects : +10.7 % (±202 %)** : non confirmé. Le benchmark est bimodal en new, à cause d'un GC majeur pendant certains échantillons (voir l'analyse du shim, `../2026-10-06/`).
- **B/op Go** (+32 octets sur NewIsolate/HeapPerIsolate/StartupWithLodash) : allocations Go marginales, sans impact.

CallJSFromGo (+2.8 %) et ValueToGoString (+2.6 %) restent sous le seuil de 5 %.

## Points revus depuis la campagne avec shim

- **CompileLodashCold** : la campagne avec shim mesurait +19.5 %, avec un A/B sans shim non concluant (±121 %). Dans cette passe, la nouvelle version est à **-8.1 %**. La régression disparaît en même temps que le shim, mais aucun A/B dans une même passe ne l'isole. **Cause de l'écart de la campagne avec shim : non établie.**
- **CompileLodashWithCodeCache** : -11.9 % (contre +21.4 % avec shim). C'est cohérent avec l'A/B de la campagne avec shim, où il revenait au niveau de la baseline sans shim.

Les preuves brutes de l'analyse du shim (strace, défauts de page, callgrind) sont dans le rapport de travail de l'analyse du shim. Le retrait lui-même (inventaire des 5 plateformes, méthode) est décrit dans le rapport du travail de retrait du shim. Ces rapports de travail ne sont pas publiés.

## Autres plateformes (CI GitHub, run 37578680646, COUNT=10)

Workflow `Botify Bench` (`.github/workflows/botify-bench.yml`), lancé sur la branche de pré-publication (un déclencheur `push` temporaire y avait été ajouté car `workflow_dispatch` répond 404 tant que le workflow n'est pas sur master ; il a été retiré ensuite). Même suite `bench/` que ci-dessus, nouvelle version compilée comme chez un consommateur (bridge précompilé), baseline = tag `v0.6.0-botify-baseline`. Sorties brutes et `benchstat` dans `ci/` (`bench-<plateforme>/*.txt`, `benchstat-<plateforme>.txt`).

**Ce sont des runners GitHub partagés (VM)** : beaucoup plus bruités que la machine de la campagne principale (écarts-types jusqu'à ±18 % sur l'Intel, ±86 % à ±181 % sur JSObjects). Les chiffres par benchmark sont à lire comme des ordres de grandeur ; seul le geomean est stable. Les runners ont 3 à 4 CPU, d'où le suffixe `-3` / `-4`.

| Plateforme | Runner | Baseline | geomean sec/op (new vs baseline) |
|---|---|---|---|
| darwin arm64 | macos-15, Apple M1 (VM), 3 CPU | oui | 136.0 µs → 127.9 µs, **-5.98 %** |
| darwin amd64 | macos-15-intel, i7-8700B, 4 CPU | oui | 381.2 µs → 336.4 µs, **-11.75 %** |
| linux arm64 | ubuntu-24.04-arm, 4 CPU | non (pas de lib V8 9.0) | 172.6 µs (nouvelle version seule) |
| windows amd64 | windows-latest, AMD EPYC 9V45, 4 CPU, clang + lld, MSVC statique | non | 124.0 µs (nouvelle version seule) |

Aucune comparaison avec la baseline n'est possible sur linux arm64 et Windows : la baseline V8 9.0 n'a pas de bibliothèque pour ces cibles. Les mesures servent de référence pour les prochains runs.

### Régressions > 5 % (p < 0.05) par rapport à la baseline

**darwin arm64** (toutes les autres lignes sont stables ou en gain, par ex. NewIsolate -42.9 %, JSONParse -61.4 %) :
- Cleanup1000Values **+96.4 %** : le coût voulu du correctif `Cleanup` (voir plus haut), +123.8 % sur linux amd64.
- JSObjects **+708.8 % (±86 %)** : le benchmark est bimodal en new, à cause d'un GC majeur pendant certains échantillons (voir l'analyse du shim, `../2026-10-06/`). Non confirmé comme régression structurelle.
- CallGoFromJS1000 **+7.8 %** (+20.9 % sur linux amd64).
- HeapPerContext **+21.8 %** (+26.5 % sur linux amd64, V8 15 alloue davantage par contexte).

**darwin amd64** (runner le plus bruité ; plusieurs lignes à ±18 % à ±36 % en baseline) :
- Cleanup1000Values **+121.6 %** : même cause.
- CompileLodashCold **+35.2 %** (±18 % en new) et CompileLodashWithCodeCache **+30.6 %** : à l'inverse de linux amd64 (-8.1 % et -11.9 %) et de darwin arm64 (-20 %). **Non expliqué** ; la campagne avec shim avait déjà vu ces deux benchmarks +20 % sur linux amd64 avec le shim, puis plus rien sans le shim. À re-mesurer sur une machine non partagée avant de conclure.
- JSObjects **+54.9 % (±181 %)** : bimodal, comme ci-dessus.
- HeapPerContext **+34.5 %**, HeapPerIsolate **+6.2 %**, JSJSON **+5.6 %**.
- CallGoFromJS1000 : pas de différence significative (~, p=0.68).

Gains sur les deux plateformes macOS : NewIsolate -43 % / -49 %, RunScriptTrivial -11 % / -46 %, JSONParse -61 % / -63 %, JSONStringify -59 % / -51 %, ObjectSetGet -26 % / -27 %.
