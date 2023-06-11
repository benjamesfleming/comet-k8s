package frontend

import (
	"net/http"

	cometdv1alpha1 "github.com/cometbackup/comet-server-operator/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Server struct {
	Client client.Client
}

func NewServer(c client.Client) *Server {
	if err := LoadTemplates(); err != nil {
		panic(err)
	}
	return &Server{c}
}

func (s *Server) ListenAndServe(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.index)

	return http.ListenAndServe(addr, mux)
}

// --

type PageData struct {
	PageTitle string
}

type IndexPageData struct {
	*PageData

	Servers []cometdv1alpha1.CometServer
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	list := &cometdv1alpha1.CometServerList{}
	err := s.Client.List(r.Context(), list)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Error"))
		return
	}
	err = Render(w, "index", &IndexPageData{
		PageData: &PageData{
			PageTitle: "Overview",
		},
		Servers: list.Items,
	})
	if err != nil {
		panic(err)
	}
}
