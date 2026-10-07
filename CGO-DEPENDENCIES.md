# Dépendances cgo et nouveau v8go

Ce document explique pourquoi certaines bibliothèques cgo ont dû être recompilées (ou « prélinkées »)
pour adopter `github.com/botify-labs/v8go`, et comment traiter les futures dépendances cgo.
Il complète `MIGRATION.md` (ce qui change pour un consommateur) et `BOTIFY.md` (le fonctionnement du
fork). L'inventaire détaillé de ftl et pulse est dans `docs/superpowers/cgo-inventory.md`.

En résumé :

- **Linux / macOS, lien dynamique (le cas normal) : rien à faire.** Les libs C, C++ (g++/libstdc++),
  précompilées ou compilées depuis leurs sources cohabitent avec V8.
- **Linux, lien totalement statique (`-extldflags '-static'`) :** une lib C++ statique construite avec
  libstdc++ entre en conflit avec le runtime C++ de V8. Les libs C ne sont pas concernées.
- **Windows :** tout est lié statiquement en ABI MSVC (`/MT`). Toute lib précompilée en MinGW doit être
  recompilée en MSVC.

## 1. Ce que le nouveau v8go apporte dans votre binaire

V8 n'est plus compilé par nos soins avec gcc : on importe les archives précompilées de
[tommie/v8go](https://github.com/tommie/v8go), construites par Chromium avec **clang**. Trois faits
en découlent.

| Fait | Conséquence |
|---|---|
| V8 embarque **le runtime C++ de Chromium** : libc++ (`libc++-cr.a`, dans le namespace `std::__Cr`) et libc++abi (`libc++abi-cr.a`) | Un second runtime C++ coexiste avec le libstdc++ de gcc (Linux) |
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
- en lien dynamique, libstdc++ est la bibliothèque partagée `libstdc++.so.6`, alors que libc++abi est
  liée statiquement dans l'exécutable : une bibliothèque partagée ne provoque jamais de
  `multiple definition`. Les deux couches ABI coexistent donc ;
- les deux couches suivent l'ABI Itanium, donc le code gcc de liburlnorm peut se lier aux `__cxa_*` de
  libc++abi sans incident (les tests de gojs passent).

### 2.2 Lien totalement statique (`-extldflags '-static'`, comme pulse) : les libs C++ g++ entrent en conflit

Dans un lien statique, libstdc++ est `libstdc++.a` (libsupc++). Chromium laisse volontairement dans les
namespaces **non versionnés** (`std::`, `__cxxabiv1::`) quelques classes de la couche ABI, pour la
compatibilité Itanium : `std::exception`, `std::bad_exception`, `std::type_info`,
`std::logic_error` / `std::runtime_error`, `std::__throw_bad_alloc`,
`__cxxabiv1::__class_type_info` / `__si_class_type_info`. libsupc++ définit les mêmes noms mangés.

Dès qu'une lib C++ construite avec g++ tire un membre de `libstdc++.a` (iostream, `std::mutex`,
locales…), l'éditeur de liens obtient deux définitions :

```text
/usr/bin/ld: libstdc++.a(eh_exception.o): in function `std::exception::~exception()':
  multiple definition of `std::exception::~exception()'; libc++abi-cr.a(stdlib_exception.o): first defined here
/usr/bin/ld: libstdc++.a(class_type_info.o): ...
  multiple definition of `__cxxabiv1::__class_type_info::~__class_type_info()'; libc++abi-cr.a(private_typeinfo.o)
/usr/bin/ld: libstdc++.a(tinfo.o): ...
  multiple definition of `std::type_info::~type_info()'; libc++abi-cr.a(stdlib_typeinfo.o)
```

(Trace réelle de l'essai gojs + liburlnorm + V8 en `-static`.) Le conflit se produit en lien statique
seulement : en dynamique, ces membres viennent de `libstdc++.so`.

**Ce qui a été fait pour liburlnorm** (C++ + ICU 65, la seule lib C++ précompilée de ftl et pulse) :
elle est **prélinkée avec une libstdc++ privée** (`liburlnorm/scripts/prelink_linux.sh` dans cdf), en
un seul objet `liburlnorm_prelinked.a`. Le script :

1. fait un `ld -r` de liburlnorm, des libs ICU et de `libstdc++.a` ;
2. ne garde globales que les 8 fonctions de l'API C (`urlnorm_new`, `urlnorm_normalize`…), avec
   `objcopy --keep-global-symbols` ; tous les symboles C++ deviennent locaux ;
3. convertit au préalable les symboles `STB_GNU_UNIQUE` (que `objcopy` ne sait pas localiser) en
   `STB_GLOBAL`, puis supprime les groupes COMDAT (`-R .group`), pour que l'éditeur de liens ne
   remplace pas notre copie privée par celle d'un autre objet.

Le résultat est épinglé par `prelink_test.go` (valeurs de référence, y compris un hôte IDN qui passe
par les données ICU) et par le job statique de pulse (`go-backend`).

**Les libs C pures** (zstd, igzip/ISA-L) ne référencent aucun symbole libstdc++ : elles ne sont pas
concernées, en dynamique comme en statique.

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
| C++ compilé depuis les sources avec g++ | OK | **Conflit** (libstdc++.a) |
| C++ précompilé avec g++/libstdc++ | OK | **Conflit** : prélinker avec une libstdc++ privée |

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
| **C++ (sources ou précompilé)** | OK | isolation nécessaire : prélink (comme liburlnorm) ou compilation depuis les sources avec clang et le même libc++ | recompiler en `/MT` avec clang-cl |

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
# des lignes   : dépendance libstdc++ (C++) : prévoir l'isolation en lien statique
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
- **C++ précompilé, lien statique Linux** : reprendre `prelink_linux.sh` de liburlnorm (voir §2.2) ;
  seule l'API C reste globale.
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
- **Éviter le C++ dans les dépendances cgo** quand un lien totalement statique est exigé ; sinon
  l'isoler (prélink Linux, `/MT` Windows) et documenter la procédure dans le dépôt de la lib.
- Une lib précompilée livre un `.a` Linux, un `.a` macOS et un `.lib` Windows `/MT` **ensemble** ; la
  construire par un workflow reproductible et tester la lib committée.

### 5.3 Développement Windows : l'alternative WSL2

Un build Linux sous **WSL2** évite toutes les contraintes MSVC (pas de clang, pas de `.lib`, toolchain
Linux habituelle). Réserver le build Windows natif aux cas où l'on cible réellement Windows ; la CI
Windows (§5.1) couvre ce besoin.

## 6. Piste structurelle (non implémentée)

Aujourd'hui, le conflit statique de §2.2 est résolu **dépendance par dépendance** (liburlnorm, côté cdf).
On pourrait l'éliminer pour **toutes** les futures dépendances C++ en isolant le runtime C++ de V8
**dans v8go lui-même**, sur Linux : prélinker en un seul objet le bridge (`libv8go.a`), V8 (`libv8-*.a`)
et libc++ / libc++abi, en n'exportant que l'API C de v8go (même méthode que `prelink_linux.sh`,
appliquée côté v8go). Plus aucun symbole `std::` ou `__cxxabiv1::` de Chromium ne serait global, donc plus
de collision avec `libstdc++.a`, quelle que soit la lib C++ liée.

Coût : une étape de construction plus lourde à chaque synchronisation de V8 (prélink de plusieurs
archives, localisation des symboles, vérifications, taille des archives), à intégrer à
`tools/build_bridge.sh` et à `botify-bridge`. Les deux couches ABI cohabiteraient en privé, ce qui est
acceptable tant que les exceptions C++ ne traversent pas la frontière (V8 est compilé sans exceptions).

## 7. Références

- `MIGRATION.md` : toolchains, glibc, mémoire, changements d'API.
- `BOTIFY.md` : fonctionnement du fork, bridges précompilés, mise à jour de V8.
- `docs/superpowers/cgo-inventory.md` : inventaire cgo de ftl et pulse (2026-10-06).
- `docs/superpowers/specs/2026-10-06-v8-upgrade-design.md` : design de la montée de version, risques.
