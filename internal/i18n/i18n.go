// Package i18n traduit les messages produits côté Go (erreurs, journal, noms
// d'opérations). La langue est choisie par l'interface ; l'anglais sert de défaut
// et de repli quand une traduction manque.
package i18n

import (
	"fmt"
	"strings"
	"sync/atomic"
)

const Default = "en"

var current atomic.Value

func init() { current.Store(Default) }

// Set change la langue courante (en, fr, es) ; une langue inconnue retombe sur l'anglais.
func Set(lang string) {
	lang = strings.ToLower(lang)
	if _, ok := messages["busy"][lang]; !ok {
		lang = Default
	}
	current.Store(lang)
}

// Lang renvoie la langue courante.
func Lang() string { return current.Load().(string) }

// T traduit une clé et formate ses arguments à la manière de fmt.Sprintf.
func T(key string, args ...any) string {
	m, ok := messages[key]
	if !ok {
		return key
	}
	s, ok := m[Lang()]
	if !ok {
		s = m[Default]
	}
	if len(args) == 0 {
		return s
	}
	return fmt.Sprintf(s, args...)
}

// Units renvoie les unités de taille et le séparateur décimal de la langue courante.
func Units() ([]string, string) {
	switch Lang() {
	case "fr":
		return []string{"o", "Ko", "Mo", "Go", "To"}, ","
	case "es":
		return []string{"B", "KB", "MB", "GB", "TB"}, ","
	}
	return []string{"B", "KB", "MB", "GB", "TB"}, "."
}

type tr = map[string]string

var messages = map[string]tr{
	// Opérations (bandeau « en cours »)
	"busy":             {"en": "another operation is already running", "fr": "une opération est déjà en cours", "es": "ya hay una operación en curso"},
	"task.analyze":     {"en": "Analyzing %s", "fr": "Analyse de %s", "es": "Analizando %s"},
	"task.trim":        {"en": "Trimming %s", "fr": "Trim de %s", "es": "Trim de %s"},
	"task.shutdown":    {"en": "Shutting down WSL", "fr": "Arrêt de WSL", "es": "Apagando WSL"},
	"task.compact":     {"en": "Compacting %s", "fr": "Compactage de %s", "es": "Compactando %s"},
	"task.backup":      {"en": "Backing up %s", "fr": "Sauvegarde de %s", "es": "Copia de seguridad de %s"},
	"task.sparse":      {"en": "Sparse mode for %s", "fr": "Mode sparse de %s", "es": "Modo sparse de %s"},
	"task.export":      {"en": "Exporting %s", "fr": "Export de %s", "es": "Exportando %s"},
	"task.reimport":    {"en": "Reimporting %s", "fr": "Réimport de %s", "es": "Reimportando %s"},
	"task.installNcdu": {"en": "Installing ncdu", "fr": "Installation de ncdu", "es": "Instalando ncdu"},
	"task.scan":        {"en": "Scanning %s", "fr": "Scan de %s", "es": "Escaneando %s"},
	"task.cleanup":     {"en": "Cleaning up", "fr": "Nettoyage", "es": "Limpiando"},
	"task.delete":      {"en": "Deleting %d item(s)", "fr": "Suppression de %d élément(s)", "es": "Eliminando %d elemento(s)"},

	// Journal
	"log.analyzed": {"en": "%s: %s on the Windows disk, %s used in Linux, up to %s reclaimable.",
		"fr": "%s : %s sur le disque Windows, %s utilisés dans Linux, %s récupérables au mieux.",
		"es": "%s: %s en el disco de Windows, %s usados en Linux, hasta %s recuperables."},
	"log.trimmed": {"en": "fstrim reported %s of free blocks; the VHDX went from %s to %s on disk.",
		"fr": "fstrim a signalé %s de blocs libres ; le fichier VHDX est passé de %s à %s sur le disque.",
		"es": "fstrim informó %s de bloques libres; el VHDX pasó de %s a %s en disco."},
	"log.shutdownBeforeCompact": {"en": "Shutting down WSL before compacting…", "fr": "Arrêt de WSL avant le compactage…", "es": "Apagando WSL antes de compactar…"},
	"log.compacted": {"en": "Compaction done in %s: %s → %s (%s reclaimed).",
		"fr": "Compactage terminé en %s : %s → %s (%s récupérés).",
		"es": "Compactación terminada en %s: %s → %s (%s recuperados)."},
	"log.backupDone":     {"en": "Backup done: %s", "fr": "Sauvegarde terminée : %s", "es": "Copia de seguridad terminada: %s"},
	"log.exportDone":     {"en": "Export done: %s", "fr": "Export terminé : %s", "es": "Exportación terminada: %s"},
	"log.reimportDone":   {"en": "Reimport done: %s → %s.", "fr": "Réimport terminé : %s → %s.", "es": "Reimportación terminada: %s → %s."},
	"log.cancel":         {"en": "Cancelling…", "fr": "Annulation demandée…", "es": "Cancelando…"},
	"log.scanDone":       {"en": "Scan done: %s, %d items, in %s.", "fr": "Scan terminé : %s, %d éléments, en %s.", "es": "Escaneo terminado: %s, %d elementos, en %s."},
	"log.cleanupDone":    {"en": "Cleaned %s: %s freed.", "fr": "Nettoyage de %s : %s libérés.", "es": "Limpieza de %s: %s liberados."},
	"log.deleted":        {"en": "%d item(s) deleted, %s freed.", "fr": "%d élément(s) supprimé(s), %s libérés.", "es": "%d elemento(s) eliminado(s), %s liberados."},
	"log.copy":           {"en": "Copying %s to %s", "fr": "Copie de %s vers %s", "es": "Copiando %s a %s"},
	"log.uac":            {"en": "%s needs administrator rights: accept the UAC prompt.", "fr": "%s doit tourner en administrateur : validez l'invite UAC.", "es": "%s necesita permisos de administrador: acepte el aviso de UAC."},
	"log.uidRestored":    {"en": "Default user restored (uid %d).", "fr": "Utilisateur par défaut restauré (uid %d).", "es": "Usuario predeterminado restaurado (uid %d)."},
	"log.uidNotRestored": {"en": "Default user not restored: %s", "fr": "Utilisateur par défaut non restauré : %s", "es": "Usuario predeterminado no restaurado: %s"},
	"log.wslconfig": {"en": "%s: [experimental] sparseVhd=%v (applies to new distributions).",
		"fr": "%s : [experimental] sparseVhd=%v (s'applique aux nouvelles distributions).",
		"es": "%s: [experimental] sparseVhd=%v (se aplica a las distribuciones nuevas)."},

	// Erreurs
	"err.noRegistry":     {"en": "no WSL distribution found in the registry", "fr": "aucune distribution WSL trouvée dans le registre", "es": "no se encontró ninguna distribución WSL en el registro"},
	"err.notInRegistry":  {"en": "distribution %s not found in the registry", "fr": "distribution %s introuvable dans le registre", "es": "distribución %s no encontrada en el registro"},
	"err.distroNotFound": {"en": "distribution %q not found", "fr": "distribution %q introuvable", "es": "distribución %q no encontrada"},
	"err.wsl1":           {"en": "WSL 1 distribution: there is no virtual disk to compact", "fr": "distribution WSL 1 : il n'y a pas de disque virtuel à compacter", "es": "distribución WSL 1: no hay disco virtual que compactar"},
	"err.stillLocked": {"en": "%s is still in use after shutting down WSL (is Docker Desktop or another tool keeping it open?)",
		"fr": "%s est toujours utilisé après l'arrêt de WSL (Docker Desktop ou un autre outil le garde ouvert ?)",
		"es": "%s sigue en uso tras apagar WSL (¿Docker Desktop u otra herramienta lo mantiene abierto?)"},
	"err.unknownMethod":  {"en": "unknown compaction method: %s", "fr": "méthode de compactage inconnue : %s", "es": "método de compactación desconocido: %s"},
	"err.uacDenied":      {"en": "%s: elevation denied (UAC prompt cancelled)", "fr": "%s : élévation refusée (invite UAC annulée)", "es": "%s: elevación denegada (aviso de UAC cancelado)"},
	"err.elevatedFailed": {"en": "%s failed (%v): see the log", "fr": "%s a échoué (%v) : voir le journal", "es": "%s falló (%v): consulte el registro"},
	"err.noSpace": {"en": "not enough space in %s: %s free, %s needed", "fr": "espace insuffisant dans %s : %s libres, %s nécessaires",
		"es": "espacio insuficiente en %s: %s libres, %s necesarios"},
	"err.managedReimport": {"en": "%s is managed by Docker Desktop: reimport refused", "fr": "%s est gérée par Docker Desktop : réimportation refusée", "es": "%s está gestionada por Docker Desktop: reimportación rechazada"},
	"err.archiveMissing":  {"en": "archive not found", "fr": "archive introuvable", "es": "archivo no encontrado"},
	"err.archiveSuspect":  {"en": "suspicious archive (%s): reimport cancelled", "fr": "archive suspecte (%s) : réimportation annulée", "es": "archivo sospechoso (%s): reimportación cancelada"},
	"err.importFailed": {"en": "%v. The archive %s is intact: reimport it with `wsl --import %s \"%s\" \"%s\"`",
		"fr": "%v. L'archive %s est intacte : réimportez-la avec `wsl --import %s \"%s\" \"%s\"`",
		"es": "%v. El archivo %s está intacto: reimpórtelo con `wsl --import %s \"%s\" \"%s\"`"},
	"err.ncduMissing":      {"en": "ncdu is not installed in %s", "fr": "ncdu absent de %s", "es": "ncdu no está instalado en %s"},
	"err.ncduRead":         {"en": "reading the ncdu export", "fr": "lecture de l'export ncdu", "es": "lectura de la exportación de ncdu"},
	"err.unknownCleanup":   {"en": "unknown cleanup: %s", "fr": "nettoyage inconnu : %s", "es": "limpieza desconocida: %s"},
	"err.notDeleted":       {"en": "%s could not be deleted: %v", "fr": "%s n'a pas pu être supprimé : %v", "es": "no se pudo eliminar %s: %v"},
	"err.notInScan":        {"en": "%s is not in the last scan", "fr": "%s n'existe pas dans le dernier scan", "es": "%s no está en el último escaneo"},
	"err.protected":        {"en": "%s is protected", "fr": "%s est protégé", "es": "%s está protegido"},
	"err.nothingToDelete":  {"en": "nothing to delete", "fr": "rien à supprimer", "es": "nada que eliminar"},
	"err.noScan":           {"en": "no scan for this distribution", "fr": "aucun scan pour cette distribution", "es": "no hay escaneo para esta distribución"},
	"err.deleteFailed":     {"en": "%d item(s) could not be deleted", "fr": "%d élément(s) n'ont pas pu être supprimés", "es": "no se pudieron eliminar %d elemento(s)"},
	"err.unexpectedDf":     {"en": "unexpected df output: %q", "fr": "sortie de df inattendue : %q", "es": "salida de df inesperada: %q"},
	"err.unexpectedFstrim": {"en": "unexpected fstrim output: %q", "fr": "sortie de fstrim inattendue : %q", "es": "salida de fstrim inesperada: %q"},

	// Boîtes de dialogue natives
	"dlg.export":    {"en": "Export %s", "fr": "Exporter %s", "es": "Exportar %s"},
	"dlg.reimport":  {"en": "Archive to reimport", "fr": "Archive à réimporter", "es": "Archivo a reimportar"},
	"dlg.tarFilter": {"en": "Tar archive (*.tar)", "fr": "Archive tar (*.tar)", "es": "Archivo tar (*.tar)"},
}
