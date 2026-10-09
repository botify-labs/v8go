package v8go

// /* V8's ICU is linked in from the deps libraries, with its symbols suffixed
//    by its major version. A V8 upgrade that changes the ICU version makes
//    the link fail on this symbol: update the suffix to the one that
//    nm deps/linux_amd64/libv8-*.a | grep 'T uloc_setDefault' shows. */
// #include <stdlib.h>
//
// typedef int UErrorCode;
// void uloc_setDefault_78(const char* localeID, UErrorCode* status);
//
// static int v8go_icu_set_default_locale(const char* locale) {
//   UErrorCode status = 0;
//   uloc_setDefault_78(locale, &status);
//   return status;
// }
import "C"

import (
	"fmt"
	"unsafe"
)

// SetDefaultLocale sets the default locale of V8's ICU, process-wide: the
// locale of Intl and of the locale-sensitive methods (toLocaleString,
// localeCompare, toLocaleDateString, the time zone name in
// Date.prototype.toString...) when a script passes none. Unset, ICU takes it
// from the host (LC_ALL, LC_MESSAGES or LANG on Linux and macOS, the user's
// settings on Windows), so the same script gives "1,234,567.891" on one host
// and "1 234 567,891" on another.
//
// locale is an ICU locale ID such as "en_US". Call it before creating any
// isolate: an isolate reads the default locale once, the first time it needs
// it. V8 has no flag for this (--icu-locale is a d8 option).
//
// The time zone is not affected: ICU detects it once per process, from TZ,
// else /etc/localtime, on Linux and macOS, and from the system settings on
// Windows.
func SetDefaultLocale(locale string) error {
	cl := C.CString(locale)
	defer C.free(unsafe.Pointer(cl))
	if status := C.v8go_icu_set_default_locale(cl); status > 0 {
		return fmt.Errorf("v8go: setting ICU's default locale to %q: UErrorCode %d", locale, int(status))
	}
	return nil
}
