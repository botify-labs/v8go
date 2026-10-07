# Dépendances cgo et nouveau v8go

Ce document explique pourquoi certaines bibliothèques cgo ont dû être recompilées (Windows) ou
« prélinkées » (Linux statique, avant l'isolation du runtime C++ de V8) pour adopter `github.com/botify-labs/v8go`, et comment traiter les futures dépendances cgo.
Il complète `MIGRATION.md` (ce qui change pour un consommateur) et `BOTIFY.md` (le fonctionnement du
fork). L'inventaire détaillé de ftl et pulse est dans `docs/superpowers/cgo-inventory.md`.

En résumé :

- **Linux / macOS, lien dynamique (le cas normal) : rien à faire.** Les libs C, C++ (g++/libstdc++),
  précompilées ou compilées depuis leurs sources cohabitent avec V8.
- **Linux, lien totalement statique (`-extldflags '-static'`) : rien à faire non plus.** Le runtime
  C++ de V8 est renommé dans ses archives Linux : les libs C++ g++/libstdc++ n'entrent plus en conflit
  avec lui (§2.2). Le prélink de liburlnorm n'est plus nécessaire.
- **Windows :** tout est lié statiquement en ABI MSVC (`/MT`). Toute lib précompilée en MinGW doit être
  recompilée en MSVC.

## 1. Ce que le nouveau v8go apporte dans votre binaire

V8 n'est plus compilé par nos soins avec gcc : on importe les archives précompilées de
[tommie/v8go](https://github.com/tommie/v8go), construites par Chromium avec **clang**. Trois faits
en découlent.

| Fait | Conséquence |
|---|---|
| V8 embarque **le runtime C++ de Chromium** : libc++ (`libc++-cr.a`, dans le namespace `std::__Cr`) et libc++abi (`libc++abi-cr.a`) | Un second runtime C++ coexiste avec le libstdc++ de gcc (Linux) ; sa couche ABI est renommée (`.v8cr`) pour ne pas entrer en collision avec lui (§2.2) |
| Sous Windows, V8 n'existe qu'en **ABI MSVC avec le CRT statique** (`/MT`, `libcmt`) | Tout le binaire doit être MSVC : pas de MinGW |
| V8 est **précompilé** (`deps/<os>_<arch>`), ainsi que le bridge C++ de v8go (`libv8go.a`, `v8go.lib` sous Windows) | Le consommateur ne compile aucun C++ pour v8go ; seules les autres dépendances cgo passent par sa toolchain |

Le bridge précompilé est lié automatiquement (`#cgo !v8go_source LDFLAGS: -lv8go`) ; le mode source
(`-tags v8go_source`) ne sert qu'à développer v8go (voir `BOTIFY.md`).

## 2. Linux et macOS

### 2.1 Lien dynamique (glibc) : tout fonctionne

Sont compatibles, sans rien changer :

- les packages cgo compilés depuis leurs sources (DataDog/zstd, `lz4`…) ;
- les libs C précompilées (zstd de gocdf, igzip/ISA-L) ;
- les libs **C++ construites avec g++/libstdc++**, par exemple liburlnorm + ICU 65 : elles cohabitent
  avec V8 dans le même binaire (testé dans gojs, y compris Intl).

Pourquoi ça marche :

- libc++ vit dans le namespace versionné `std::__Cr` : ses symboles (`std::__Cr::basic_string`…)
  ne peuvent pas entrer en collision avec ceux de libstdc++ (`std::`, `std::__cxx11::`) ;
- la couche ABI de Chromium (`__cxa_*`, `std::exception`…), qui porte les mêmes noms que celle de
  libstdc++, est renommée dans les archives Linux de v8go (suffixe `.v8cr`, §2.2) : le code g++ se lie
  au runtime de `libstdc++.so.6`, V8 au sien, sans interférence.

Avant ce renommage, l'exécutable définissait `__cxa_throw`, `__gxx_personality_v0`, les typeinfo `std::`…
de libc++abi, qui **supplantaient** ceux de `libstdc++.so` : le lien réussissait, mais une exception
levée à l'intérieur de libstdc++ (`std::stoi("x")`, par exemple) et attrapée par du code g++ finissait en
`std::terminate`. Les tests de gojs (liburlnorm) ne passaient pas par ce chemin ; `internal/cxxprobe` le
reproduit.

### 2.2 Lien totalement statique (`-extldflags '-static'`, comme pulse) : fonctionne, libs C++ g++ comprises

Dans un lien statique, libstdc++ est `libstdc++.a` (libsupc++). Chromium laisse volontairement hors de
son namespace privé `std::__Cr` la couche ABI C++, pour la compatibilité Itanium : `__cxa_*`,
`__gxx_personality_v0`, `__dynamic_cast`, `std::exception` et les autres exceptions `std::`
(`std::bad_alloc`, `std::logic_error`, `std::runtime_error`…), `std::type_info`, les typeinfo et vtables
de `__cxxabiv1::__class_type_info` & co, `std::terminate`, `operator new`/`delete`. libstdc++ et
libsupc++ définissent les mêmes noms mangés.

**Avant l'isolation**, dès qu'une lib C++ construite avec g++ tirait un membre de `libstdc++.a`,
l'éditeur de liens obtenait deux définitions (28 `multiple definition`, trace réelle de gojs +
liburlnorm + V8 en `-static`) :

```text
/usr/bin/ld: libstdc++.a(eh_exception.o): in function `std::exception::~exception()':
  multiple definition of `std::exception::~exception()'; libc++abi-cr.a(stdlib_exception.o): first defined here
/usr/bin/ld: libstdc++.a(class_type_info.o): ...
  multiple definition of `__cxxabiv1::__class_type_info::~__class_type_info()'; libc++abi-cr.a(private_typeinfo.o)
```

**Isolation du runtime C++ de V8** (modules `deps/linux_*` à partir de
`v0.0.0-20261007203548-1be5d95a805e`, que le `go.mod` de v8go épingle) : dans les archives Linux (`libv8-*.a`, `libc++-cr.a`,
`libc++abi-cr.a`, le bridge `libv8go.a`), tous ces symboles sont **renommés**, définitions et
références, avec le suffixe `.v8cr` (`objcopy --redefine-syms`) :

- la table (`tools/cxx-runtime-rename.map`, 499 symboles) est dérivée mécaniquement des archives par
  `tools/gen_cxx_runtime_rename_map.sh` : tout symbole global défini par `libc++-cr.a` ou
  `libc++abi-cr.a` hors des namespaces privés `__Cr` et `__llvm_libc_cr`. Elle ne dépend pas de la
  version de gcc du consommateur (gcc 10 à 14 en définissent 399 à 400 ; les autres sont aussi
  renommés, pour que le code g++ ne se lie jamais par erreur à une définition de Chromium) ;
- le groupe COMDAT `DW.ref.__gxx_personality_v0` est renommé aussi : les frames de V8 et celles du code
  g++ gardent chacune **leur** routine de personnalité, donc leurs exceptions et leur RTTI ;
- `_Unwind_*` (l'unwinder de libgcc, partagé et compatible), la libc et les builtins de compiler-rt
  (fonctions sans état, une par membre) ne sont pas renommés ;
- aucun coût à l'exécution : seuls les noms changent (benchmarks inchangés).

Après le renommage, le binaire contient les deux runtimes côte à côte (`__cxa_throw` de libsupc++ et
`__cxa_throw.v8cr` de V8). C'est vérifié à chaque push par le job `static-cxx-probe` de botify-ci
(`internal/cxxprobe` : du C++ g++ qui lève et attrape `std::runtime_error`, `std::bad_alloc`,
`std::invalid_argument` venue de libstdc++, utilise RTTI et iostream, lié en `-static` avec v8go sur
amd64 et arm64), et par gojs lié en `-static` avec la liburlnorm **d'origine** (non prélinkée).

**liburlnorm** (C++ + ICU 65, la seule lib C++ précompilée de ftl et pulse) avait été **prélinkée avec
une libstdc++ privée** (`liburlnorm/scripts/prelink_linux.sh` dans cdf, `liburlnorm_prelinked.a`, seule
l'API C reste globale) pour contourner ce conflit. Ce prélink **n'est plus nécessaire** : gojs se lie
en `-static` et ses tests passent avec `-lurlnorm -licui18n -licuio -licutu -licuuc -licudata -ldl`. Il
reste **sans danger** (sa libstdc++ est privée) et peut être gardé. Une future lib C++ n'a rien à faire
de particulier.

**Les libs C pures** (zstd, igzip/ISA-L) ne référencent aucun symbole libstdc++ : elles ne sont pas
concernées, en dynamique comme en statique.

**Limites du mode source** (`-tags v8go_source`, développement de v8go) : il n'est pas isolé
(l'exécutable exporte les noms d'origine), donc du code g++/libstdc++ lié avec lui n'est pas
supporté ; sous ASan, la détection des `new`/`delete` non appariés est perdue pour le code de V8 et
de v8go. Le mode consommateur (par défaut) est isolé : voir `BOTIFY.md`.

**Comportement qui change pour les consommateurs** : `std::set_terminate`, `std::set_new_handler` et
un `operator new` global remplacé dans votre code n'agissent plus sur V8 (en lien dynamique, ils
l'atteignaient avant l'isolation, par interposition des noms). Ils agissent toujours sur votre propre
C++.

### 2.3 Plancher glibc

- À l'exécution : **glibc ≥ 2.34** (Amazon Linux 2023 convient ; Amazon Linux 2 n'est plus supporté).
- Le plancher dépend de l'**hôte de build** : les archives V8 référencent des symboles libm sans
  version (`fmod`…), liés à la version par défaut de la glibc de l'hôte. Lié sur une glibc ≥ 2.38, le
  binaire exige `fmod@GLIBC_2.38` et ne démarre pas sur AL2023. **Construire sur la plus ancienne cible**
  (`amazonlinux:2023`). Vérification :

```sh
objdump -T ./binaire | grep -oE 'GLIBC_[0-9.]+' | sort -Vu | tail -1   # au plus GLIBC_2.34
```

Un lien entièrement statique n'est pas concerné.

### 2.4 Récapitulatif Linux / macOS

| Type de dépendance | Lien dynamique | Lien totalement statique (Linux) |
|---|---|---|
| Go pur | OK | OK |
| C compilé depuis les sources | OK | OK |
| C précompilé (`.a`) | OK | OK |
| C++ compilé depuis les sources avec g++ | OK | OK (runtime C++ de V8 renommé, §2.2) |
| C++ précompilé avec g++/libstdc++ | OK | OK (un prélink comme celui de liburlnorm n'est plus nécessaire) |

macOS n'a pas de lien totalement statique : la ligne de droite ne s'y applique pas.

## 3. Windows (natif)

### 3.1 Pourquoi tout passe en MSVC

- V8 n'existe qu'en **ABI MSVC**, et ne supporte plus MinGW.
- Un binaire statique ne peut contenir qu'**un seul runtime C**. V8 impose `/MT` (`libcmt`) : le binaire
  entier est donc MSVC. Cela implique Go ≥ 1.27 et `CC="clang -fuse-ld=lld"` (clang ≥ 21 ciblant MSVC,
  MSVC Build Tools + Windows SDK).
- Pas de repli « MinGW + DLL » : tout doit être statique (décision de conception) et V8 ne se compile
  plus en MinGW de toute façon.

### 3.2 Ce que clang-MSVC change pour vos dépendances

- **Packages cgo compilés depuis leurs sources** : recompilés automatiquement par clang ciblant MSVC.
  La plupart du C passe tel quel. Du code spécifique GCC/MinGW peut échouer : `pthread.h`, `unistd.h`,
  extensions `__attribute__`, en-têtes réservés à MinGW. À vérifier lors de l'ajout d'une dépendance.
- **Libs précompilées en MinGW (`.a`) : elles doivent être recompilées en MSVC `/MT` (`.lib`).** Elles
  référencent le runtime MinGW (`___chkstk_ms`, `__mingw_vfprintf`, `__mingw_vsnprintf`…), que le CRT
  MSVC ne fournit pas.

Ce qui a été recompilé (voir `docs/superpowers/cgo-inventory.md`) :

| Lib | Dépôt | Résultat | Remarques |
|---|---|---|---|
| liburlnorm + ICU 65 | cdf | `liburlnorm/go/urlnorm/lib/windows_msvc/` (`urlnorm.lib`, `sicu{uc,in,io,tu,dt}.lib`) | ICU 65 conservée (normalisation identique à la production) ; workflow `windows-msvc-libs.yml` ; C++ (`libcpmt`) |
| igzip (ISA-L) | cdf et pulse | `go/pkg/igzip/lib/windows/igzip.lib` dans chaque dépôt | ISA-L 2.31.1 (cdf) et 2.30.0 (pulse) ; assemblé avec NASM |
| zstd | gocdf | `compress/zstd/lib/zstd_windows.lib` (PR #926) | zstd 1.5.0, clang-cl, build reproductible ; à publier dans une version de gocdf |

Dans chaque cas, un workflow construit la lib sur un runner Windows, vérifie les symboles et le CRT,
puis exécute `go test` avec `CC="clang -fuse-ld=lld"`, d'abord contre la `.lib` commitée, puis contre
une reconstruction.

**Astuce de coexistence.** Avec clang ciblant MSVC, `-lfoo` résout `foo.lib` ; avec MinGW, `-lfoo`
résout `libfoo.a`. On place donc `foo.lib` à côté de `libfoo.a` dans le même dossier `-L` : les
`LDFLAGS` ne changent pas et les builds MinGW restent possibles.

```text
go/pkg/igzip/lib/windows/
├── libigzip.a   # MinGW, ancien
└── igzip.lib    # MSVC /MT, nouveau ; `-ligzip` prend l'un ou l'autre selon la cible
```

(Exception : liburlnorm, dont les libs MSVC sont dans un dossier distinct, `lib/windows_msvc` ; ses
`LDFLAGS` Windows y pointent (avec `-ladvapi32`) et les anciennes `.a` MinGW ont été retirées.)

## 4. Ajouter un nouveau module cgo tiers : check-list

| Type de dépendance | Linux / macOS (dynamique) | Linux, lien totalement statique | Windows (MSVC) |
|---|---|---|---|
| **Go pur** | rien à faire | rien à faire | rien à faire |
| **C compilé depuis les sources** | OK | OK | vérifier qu'il compile avec clang-MSVC |
| **C, lib statique précompilée** | OK | OK | il faut un `.lib` construit en `/MT` |
| **C++ (sources ou précompilé)** | OK | OK (runtime C++ de V8 isolé, §2.2) | recompiler en `/MT` avec clang-cl |

### 4.1 Détecter chaque cas

**Inventaire des packages cgo** (forme utilisée pour `cgo-inventory.md`, à lancer par OS cible, module
par module) :

```sh
for os in linux windows; do
  echo "## $os"
  GOOS=$os GOARCH=amd64 CGO_ENABLED=1 go list -deps \
    -f '{{if .CgoFiles}}{{.ImportPath}} | C++ : {{.CXXFiles}} | LDFLAGS : {{.CgoLDFLAGS}}{{end}}' ./...
done
```

Lecture de la sortie :

- `.CgoFiles` non vide : package cgo ;
- `-L...` dans les LDFLAGS : lib **précompilée** (à auditer) ; sinon, compilée depuis les sources ;
- `.CXXFiles` non vide, ou `-lstdc++` / ICU / bibliothèque C++ connue : C++ (même `dummy.cpp` : il force
  l'éditeur de liens C++, comme dans liburlnorm).

**Une lib statique précompilée référence-t-elle libstdc++ ?** (Linux ; `libc++` de V8 est dans
`std::__Cr`, on l'exclut.)

```sh
nm -A --undefined-only libfoo.a | grep -E '_ZNSt|_ZSt|_ZNKSt|__cxa_|__gxx_personality' | grep -v '__Cr' | head
# aucune ligne : C pur, pas de risque de conflit en lien statique
# des lignes   : dépendance libstdc++ (C++) : sans conséquence depuis l'isolation du runtime de V8 (§2.2)
```

**Une lib Windows est-elle MinGW ou MSVC ? Quel CRT ?** (avec les outils LLVM, ou `dumpbin` si MSVC
est installé) :

```sh
# MinGW ? des symboles de runtime MinGW non résolus
llvm-nm --undefined-only libfoo.a | grep -E '__mingw_|___chkstk_ms'

# CRT demandé par une lib MSVC : directives /DEFAULTLIB
llvm-readobj --coff-directives foo.lib | grep -i defaultlib      # équivalent de : dumpbin /directives foo.lib
llvm-objdump -s -j .drectve foo.lib | head                       # même information, en hexadécimal/ASCII
```

Attendu pour une lib compatible : `/defaultlib:libcmt` (plus `libcpmt` pour du C++) et `oldnames`.
`msvcrt`, `msvcprt` ou les variantes debug (`libcmtd`) signalent un `/MD` ou un debug : à reconstruire.
Une lib sans directive (données seules, comme `sicudt.lib`) est normale.

**Le binaire respecte-t-il le plancher glibc ?** Voir §2.3.

### 4.2 Traiter chaque cas

- **C précompilé, Windows** : construire un `.lib` `/MT` (clang-cl ou `cl /MT`), le placer à côté du
  `.a`, ajouter un workflow Windows qui teste la lib committée puis une reconstruction, comme pour
  zstd et igzip.
- **C++, lien statique Linux** : rien à faire (§2.2). Le prélink de liburlnorm (`prelink_linux.sh`)
  reste une option pour masquer les symboles d'une lib, il n'est plus requis pour cohabiter avec V8.
- **C++ Windows** : reconstruire en `/MT` avec clang-cl (liburlnorm + ICU 65 en est l'exemple ; vérifier
  l'absence de symboles `_7x` si l'ICU est dupliquée avec celle de V8, qui utilise le suffixe `_78`).
- **Plusieurs ICU** : l'ICU de V8 (suffixe `_78`) et celle de liburlnorm (`_65`) coexistent grâce à ces
  suffixes. Ne pas faire passer liburlnorm en ICU 78 sans un chantier dédié (doublons de symboles et
  changement de normalisation).

## 5. Se protéger : prévention

### 5.1 Garde-fous CI dans les dépôts consommateurs

À mettre en place côté consommateur (ce dépôt n'ajoute pas de workflow dans les autres) :

1. **Un job de lien totalement statique Linux** (`-tags 'netgo osusergo'
   -ldflags "-linkmode external -extldflags '-static -lm'"`). C'est déjà le cas de `go-backend` dans
   pulse : c'est ce job qui attrape les conflits libstdc++/libc++abi.
2. **Un job Windows MSVC** si Windows compte (`CC="clang -fuse-ld=lld"`, Go ≥ 1.27), comme `gojs-windows`
   dans cdf. Sans lui, une lib MinGW oubliée n'est détectée que par un développeur Windows.
3. **Un contrôle du plancher glibc**, dans `amazonlinux:2023` (job `glibc-floor` de botify-ci, ou
   build/test dans cette image comme cbjsex).
4. **Un job « inventaire cgo »** qui échoue quand un NOUVEAU package cgo avec lib précompilée ou C++
   apparaît, pour qu'il soit relu. Principe : exécuter la commande du §4.1 pour Linux et Windows, ne
   garder que les packages à `-L` ou à `.CXXFiles` non vide, et comparer à une liste validée
   (`cgo-allowlist.txt`) ; toute différence fait échouer le job et renvoie à ce document.

Esquisse du script (à adapter, non fournie en workflow) :

```sh
#!/bin/sh
# Échoue si un package cgo « à risque » (lib précompilée -L, ou C++) n'est pas dans la liste validée.
set -eu
for os in linux windows; do
  GOOS=$os GOARCH=amd64 CGO_ENABLED=1 go list -deps \
    -f '{{if .CgoFiles}}{{if or .CXXFiles (gt (len .CgoLDFLAGS) 0)}}{{.ImportPath}}{{end}}{{end}}' ./... \
    | sort -u | sed "s/^/$os /"
done > /tmp/cgo-current.txt
# Ne retient que ce qui n'est pas déjà validé (ligne « os importpath »).
if ! diff -u cgo-allowlist.txt /tmp/cgo-current.txt; then
  echo "Nouvelle dépendance cgo : lire v8go/CGO-DEPENDENCIES.md avant de l'ajouter à cgo-allowlist.txt" >&2
  exit 1
fi
```

Ce filtre est volontairement large (`CgoLDFLAGS` non vide inclut aussi `-lpthread` de `runtime/cgo`) :
la liste validée absorbe ces entrées une fois pour toutes, et seules les nouveautés déclenchent une
revue. Un filtre plus fin sur `-L` est possible si la liste devient bruyante.

### 5.2 Politique

- **Préférer les modules Go purs.**
- **Préférer le C compilé depuis les sources aux libs précompilées** : cgo le recompile avec la toolchain
  du build (gcc, clang-MSVC), sans lib à reconstruire par plateforme.
- **Le C++ dans les dépendances cgo** est possible en lien totalement statique sous Linux (§2.2) ; sous
  Windows, il faut le reconstruire en `/MT` et documenter la procédure dans le dépôt de la lib.
- Une lib précompilée livre un `.a` Linux, un `.a` macOS et un `.lib` Windows `/MT` **ensemble** ; la
  construire par un workflow reproductible et tester la lib committée.

### 5.3 Développement Windows : l'alternative WSL2

Un build Linux sous **WSL2** évite toutes les contraintes MSVC (pas de clang, pas de `.lib`, toolchain
Linux habituelle). Réserver le build Windows natif aux cas où l'on cible réellement Windows ; la CI
Windows (§5.1) couvre ce besoin.

## 6. Isolation du runtime C++ de V8 (implémentée)

Le conflit statique de §2.2 était résolu **dépendance par dépendance** (prélink de liburlnorm, côté cdf).
Il est désormais éliminé pour **toutes** les dépendances C++, dans v8go lui-même, sur Linux.

La piste initiale était de prélinker en un seul objet le bridge, V8 et libc++/libc++abi, en
n'exportant que l'API C de v8go. Elle a été écartée : l'objet unique dépasserait la limite de 100 Mo
par fichier de GitHub, et il aurait fallu localiser des centaines de milliers de symboles à chaque
import. Le **renommage** retenu garde les archives découpées et ne touche que les ~500 symboles de la
couche ABI :

| Élément | Rôle |
|---|---|
| `tools/gen_cxx_runtime_rename_map.sh` | dérive la table des archives (`llvm-nm`), de façon déterministe ; échoue sur un nom hors des motifs ABI C++ (jamais `_Unwind_*` ni la libc) |
| `tools/cxx-runtime-rename.map` | la table committée : `nom nom.v8cr`, 499 lignes |
| `tools/rename_cxx_runtime.sh` | l'applique (`objcopy --redefine-syms`) aux archives V8, libc++, compiler-rt et aux bridges Linux ; idempotent |
| `tools/check_cxx_runtime_isolated.sh` | échoue si une archive Linux définit ou référence un nom d'origine, ou si la table n'est plus à jour |
| `tools/sync_tommie.sh`, `tools/build_bridge.sh` | appliquent le renommage à chaque import de V8 et à chaque construction de bridge ; la table et les scripts entrent dans l'empreinte des bridges |
| `deps/linux_*/libv8go_cxxalias.a`, `botify_cxxalias_linux.go` | mode source seulement (`-tags v8go_source`) : alias des noms d'origine pour les objets compilés par cgo (script d'édition de liens et `--wrap` de `operator new`/`delete`) |
| `internal/cxxprobe`, job `static-cxx-probe` | preuve en CI : C++ g++ et v8go dans un binaire entièrement statique, amd64 et arm64 |

Les deux couches ABI cohabitent sans se voir : chaque runtime lève, attrape et compare ses propres
exceptions et typeinfo. Une exception C++ ne doit toujours pas traverser la frontière entre le code
g++ et V8 (elle ne le fait pas : l'API de v8go est en C, et V8 n'en lève pas vers l'appelant).

## 7. Références

- `MIGRATION.md` : toolchains, glibc, mémoire, changements d'API.
- `BOTIFY.md` : fonctionnement du fork, bridges précompilés, mise à jour de V8.
- `docs/superpowers/cgo-inventory.md` : inventaire cgo de ftl et pulse (2026-10-06).
- `docs/superpowers/specs/2026-10-06-v8-upgrade-design.md` : design de la montée de version, risques.
