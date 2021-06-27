package content

import (
	"fmt"
	"net/http"
)

type Handler struct {
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	// TODO: render page based on CMS page lookups, etc. vugu server-side

	fmt.Fprintf(w, "TESTING!")

}
