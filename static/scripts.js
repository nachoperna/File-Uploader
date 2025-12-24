function verifyLimits(input) {
      const decenasPorMB = 5; // limite de 5 MB por archivo
      const limite = ((1024) ** 2) * decenasPorMB;
      const archivos = input.files;
      for (const archivo of archivos) {
            if (archivo.size > limite) {
                  alert(`El archivo ("${archivo.name}") excede el peso permitido de 5 MB`);
                  input.value = "";
                  return
            }
      }
      document.getElementById('acciones').classList.remove('oculto');
}

function cancel() {
      document.getElementById('upload-input').value = "";
}
