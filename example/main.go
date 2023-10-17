package main

import (
	"log"

	"github.com/ecwid/control"
	"github.com/ecwid/control/backoff"
)

func main() {
	session, dfr := control.Take("--no-startup-window")
	defer dfr()

	err := session.Navigate("https://zoid.ecwid.com")
	if err != nil {
		panic(err)
	}

	val := backoff.Value(func() (string, error) {
		return session.Query(".pager__count-pages").GetTextContent()
	})
	log.Println(val)

	backoff.Exec(func() error {
		return session.Query(`.pager__count-pages`).Click()
	})

	//log.Println(val, reflect.TypeOf(val).String())
	//frame := session.Frame()
	//err = frame.Query(`#ec-cart-email-input`).SetValue("zoid@ecwid.com")
	//if err != nil {
	//	panic(err)
	//}
	//err = frame.Query(`#form-control__checkbox--agree`).Click()
	//if err != nil {
	//	panic(err)
	//}
	//try.Do(frame.Query(".ec-cart__button--checkout").Click)
	//
	//err = frame.Query(".ec-cart__button--checkout").Click()
	//if err != nil {
	//	panic(err)
	//}
	//val.SetValue("zoid@ecwid.com")

	//page := v2.Page(session)

	//time.Sleep(time.Second * 15)
	////time.Sleep(time.Second * 2)
	//value, err := runtime.Evaluate(session, runtime.EvaluateArgs{
	//	//Expression: `document.querySelector("a[href='#method-activateTarget']")`,
	//	Expression: `document.querySelectorAll("a")`,
	//	SerializationOptions: &runtime.SerializationOptions{
	//		Serialization: "deep",
	//	},
	//})
	//if err != nil {
	//	panic(err)
	//}
	//log.Println(value)
	//err = session.Call("", nil, nil)
	//if err != nil {
	//	panic(err)
	//}
}

//func main() {
//
//	chromium, err := chrome.Launch(context.TODO(), "--disable-popup-blocking") // you can specify more startup parameters for chrome
//	if err != nil {
//		panic(err)
//	}
//	defer chromium.Close()
//	ctrl := control.New(chromium.GetClient())
//	ctrl.Client.Timeout = time.Second * 60
//
//	go func() {
//		s1, err := ctrl.CreatePageTarget("")
//		if err != nil {
//			panic(err)
//		}
//		cancel := s1.Subscribe("Page.domContentEventFired", func(e transport.Event) error {
//			v, err1 := s1.Page().GetNavigationEntry()
//			log.Println(v)
//			log.Println(err1)
//			return err1
//		})
//		defer cancel()
//		if err = s1.Page().Navigate("https://google.com/", control.LifecycleIdleNetwork, time.Second*60); err != nil {
//			panic(err)
//		}
//	}()
//
//	session, err := ctrl.CreatePageTarget("")
//	if err != nil {
//		panic(err)
//	}
//
//	var page = session.Page() // main frame
//	err = page.Navigate("https://surfparadise.ecwid.com/", control.LifecycleIdleNetwork, time.Second*60)
//	if err != nil {
//		panic(err)
//	}
//
//	_ = session.Activate()
//
//	items, err := page.QuerySelectorAll(".grid-product__title-inner")
//	if err != nil {
//		panic(err)
//	}
//	for _, i := range items {
//		title, err := i.GetText()
//		if err != nil {
//			panic(err)
//		}
//		log.Print(title)
//	}
//}
