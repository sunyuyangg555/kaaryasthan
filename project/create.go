package controller

import (
	"log"
	"net/http"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/google/jsonapi"
	"github.com/kaaryasthan/kaaryasthan/project/model"
	"github.com/kaaryasthan/kaaryasthan/user/model"
)

// CreateHandler creates project
func (c *Controller) CreateHandler(w http.ResponseWriter, r *http.Request) {
	tkn := r.Context().Value("user").(*jwt.Token)
	userID := tkn.Claims.(jwt.MapClaims)["sub"].(string)

	w.Header().Set("Content-Type", jsonapi.MediaType)
	w.WriteHeader(http.StatusCreated)

	usr := &user.User{ID: userID}
	if err := c.uds.Valid(usr); err != nil {
		log.Println("Couldn't validate user: "+usr.ID, err)
		http.Error(w, "Couldn't validate user: "+usr.ID, http.StatusUnauthorized)
		return
	}

	prj := new(project.Project)
	if err := jsonapi.UnmarshalPayload(r.Body, prj); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := c.ds.Create(usr, prj); err != nil {
		log.Println("Unable to save data: ", err)
		return
	}

	if err := jsonapi.MarshalPayload(w, prj); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
