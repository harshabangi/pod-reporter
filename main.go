package main

import (
	"context"
	"fmt"
	"github.com/carlmjohnson/requests"
	"github.com/labstack/echo/v4"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"log"
	"net/http"
	"time"
)

type statusContent struct {
	StageName         string `json:"stage_name"`
	StartTime         string `json:"start_time"`
	Duration          string `json:"duration"`
	IsCompleted       bool   `json:"is_completed"`
	InputRecordCount  int64  `json:"input_record_count"`
	OutputRecordCount int64  `json:"output_record_count"`
	TotalTasks        int    `json:"total_tasks"`
	CompletedTasks    int    `json:"completed_tasks"`
	InProgressTasks   int    `json:"in_progress_tasks"`
	ETA               string `json:"eta"`
}

func getStatusContent(c echo.Context) error {
	log.Printf("received request with params: %s, %s", c.Param("namespace"), c.Param("pod_name"))

	cl, err := InClusterClient()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError,
			fmt.Sprintf("error constructing cluster client: %s", err.Error()),
		)
	}

	pod, err := cl.CoreV1().
		Pods(c.Param("namespace")).
		Get(context.TODO(), c.Param("pod_name"), meta.GetOptions{})

	if err != nil {
		return echo.NewHTTPError(
			http.StatusInternalServerError, fmt.Sprintf("error getting pod: %s", err.Error()),
		)
	}

	var (
		url      = fmt.Sprintf("http://%s:8081/status", pod.Status.PodIP)
		response []statusContent
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = requests.
		URL(url).
		Accept("application/json").
		ContentType("application/json").
		ToJSON(&response).
		Fetch(ctx)

	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, response)
}

func InClusterClient() (*kubernetes.Clientset, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(cfg)
}

func main() {
	e := echo.New()
	e.GET("/v1/namespaces/:namespace/pods/:pod_name/status", getStatusContent)
	e.Logger.Fatal(e.Start(":8080"))
}
