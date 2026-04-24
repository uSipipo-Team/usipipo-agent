package security

import (
	"fmt"
	"os"
)

// CheckFilePermissions verifica que un archivo tenga permisos seguros
// Por defecto, espera 0600 (rw-------). Permite relajar si strict=false.
func CheckFilePermissions(filePath string, strict bool) error {
	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("cannot stat file %s: %w", filePath, err)
	}

	mode := info.Mode()
	expectedMode := os.FileMode(0600)

	// En modo no estricto, aceptamos permisos más amplios siempre que no sean mundo-legibles
	if !strict {
		// Verificar que no tengan permiso de lectura/escritura para "others" (grupo/otros)
		// Un permiso como 0644 es aceptable en modo no-estricto si no hay riesgo
		// Pero 0666 (permisos mundo-escritura) NO es aceptable nunca
		if mode.Perm()&0077 != 0 { // Tiene permiso para grupo/otros?
			return fmt.Errorf("file %s has group/other permissions (mode %04o), expected 0600 or more restrictive", filePath, mode.Perm())
		}
		return nil
	}

	// Modo estricto: debe ser exactamente 0600 (o más restrictivo)
	if mode.Perm() != expectedMode && mode.Perm()&0077 != 0 {
		return fmt.Errorf("file %s has insecure permissions %04o, expected 0600 (rw-------)", filePath, mode.Perm())
	}

	// También rechazamos si tiene permiso de escritura para grupo/otros
	if mode.Perm()&0022 != 0 {
		return fmt.Errorf("file %s has world-writable permissions, which are insecure", filePath)
	}

	return nil
}

// IsWorldReadable returns true if file is readable by others (group/other have read bit)
func IsWorldReadable(filePath string) (bool, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return false, err
	}
	perm := info.Mode().Perm()
	return perm&0004 != 0, nil // Others read bit
}

// IsWorldWritable returns true if file is writable by others
func IsWorldWritable(filePath string) (bool, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return false, err
	}
	perm := info.Mode().Perm()
	return perm&0002 != 0, nil // Others write bit
}

// SecureFileMode returns the recommended secure file mode (0600)
func SecureFileMode() os.FileMode {
	return 0600
}

// CheckEnvFilePermissions checks .env file permissions and logs/handles accordingly
// Returns error if file exists and has insecure permissions in strict mode
func CheckEnvFilePermissions(envPath string, strict bool) error {
	// If .env doesn't exist, skip (not an error)
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		return nil
	}

	return CheckFilePermissions(envPath, strict)
}

// CheckConfigDirectoryPermissions checks that config directory is not world-accessible
func CheckConfigDirectoryPermissions(dirPath string) error {
	info, err := os.Stat(dirPath)
	if err != nil {
		return fmt.Errorf("cannot stat directory %s: %w", dirPath, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("path %s is not a directory", dirPath)
	}

	perm := info.Mode().Perm()
	// Directories should not be world-accessible (read/execute for others)
	// At minimum, should be 0700 or 0750 (no world access)
	if perm&0007 != 0 {
		return fmt.Errorf("config directory %s has insecure permissions %04o (world-accessible), expected 0700 or 0750", dirPath, perm)
	}

	return nil
}
